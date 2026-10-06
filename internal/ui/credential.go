package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hussaratkuro/hyprvpn/internal/openvpn3"
)

type vpnCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// resolveVPNCredential asks the local password manager CLI (hyprvault or
// gopass, they share the vault) for the entry referenced by ref.
func resolveVPNCredential(ref, master string) (vpnCredential, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	executable, err := findVaultCLI()
	if err != nil {
		return vpnCredential{}, err
	}
	command := exec.CommandContext(ctx, executable, "credential", "get", "--ref", ref, "--password-stdin")
	command.Stdin = strings.NewReader(master + "\n")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return vpnCredential{}, fmt.Errorf("resolve %q: %s", ref, message)
	}
	var credential vpnCredential
	if err := json.Unmarshal(stdout.Bytes(), &credential); err != nil {
		return vpnCredential{}, fmt.Errorf("decode response: %w", err)
	}
	return credential, nil
}

func credentialWantsUsername(request openvpn3.InputRequest) bool {
	label := strings.ToLower(request.Name + " " + request.Description)
	return !request.Hidden && (strings.Contains(label, "user") || strings.Contains(label, "login"))
}

func findVaultCLI() (string, error) {
	for _, name := range []string{"hyprvault", "gopass"} {
		if executable, err := exec.LookPath(name); err == nil {
			return executable, nil
		}
		if current, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(current), name)
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("hyprvault or gopass was not found in PATH or beside hyprvpn")
}

// credentialRefsPath is shared with the charmvpn TUI so links work in both.
func credentialRefsPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "hyprvpn", "credential-refs.json"), nil
}

func loadCredentialRefs() (map[string]string, error) {
	result := make(map[string]string)
	path, err := credentialRefsPath()
	if err != nil {
		return result, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return make(map[string]string), err
	}
	return result, nil
}

func saveCredentialRefs(refs map[string]string) error {
	path, err := credentialRefsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(refs, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".credential-refs-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}
