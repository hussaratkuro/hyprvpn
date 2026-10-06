package desktop

import (
	"errors"
	"net/url"
	"os/exec"
)

func OpenBrowser(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return errors.New("invalid OAuth URL")
	}
	return startFirst(
		[]string{"xdg-open", rawURL},
		[]string{"gio", "open", rawURL},
	)
}

func startFirst(candidates ...[]string) error {
	var lastErr error
	for _, candidate := range candidates {
		if len(candidate) == 0 {
			continue
		}
		command := exec.Command(candidate[0], candidate[1:]...)
		if err := command.Start(); err == nil {
			go func() { _ = command.Wait() }()
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no supported desktop application is available")
	}
	return lastErr
}
