package openvpn3

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	configurationService = "net.openvpn.v3.configuration"
	configurationPath    = dbus.ObjectPath("/net/openvpn/v3/configuration")
	configurationIface   = "net.openvpn.v3.configuration"

	sessionsService = "net.openvpn.v3.sessions"
	sessionsPath    = dbus.ObjectPath("/net/openvpn/v3/sessions")
	sessionsIface   = "net.openvpn.v3.sessions"
)

var ErrSystemBus = errors.New("cannot connect to the system D-Bus")

type State int

const (
	Disconnected State = iota
	Connecting
	WaitingForBrowser
	Connected
	Paused
	Disconnecting
	Failed
)

type Profile struct {
	Path        dbus.ObjectPath
	Name        string
	Persistent  bool
	SessionPath dbus.ObjectPath
	State       State
	Status      string
	AuthURL     string
	Device      string
	Protocol    string
	ServerIP    string
	ServerPort  uint32
	CreatedAt   time.Time
	BytesIn     uint64
	BytesOut    uint64
}

type InputRequest struct {
	Type        uint32
	Group       uint32
	ID          uint32
	Name        string
	Description string
	Hidden      bool
}

type ConnectResult struct {
	SessionPath dbus.ObjectPath
	Requests    []InputRequest
}

type Client struct {
	conn *dbus.Conn
}

func New() (*Client, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSystemBus, err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Profiles() ([]Profile, error) {
	var configPaths []dbus.ObjectPath
	err := c.conn.Object(configurationService, configurationPath).
		Call(configurationIface+".FetchAvailableConfigs", 0).
		Store(&configPaths)
	if err != nil {
		return nil, friendlyError("fetching profiles", err)
	}

	sessions, err := c.sessionsByConfig()
	if err != nil {
		return nil, err
	}

	profiles := make([]Profile, 0, len(configPaths))
	for _, path := range configPaths {
		obj := c.conn.Object(configurationService, path)
		var name string
		if err := getProperty(obj, configurationIface, "name", &name); err != nil {
			continue
		}
		var persistent bool
		_ = getProperty(obj, configurationIface, "persistent", &persistent)

		profile := Profile{Path: path, Name: name, Persistent: persistent, State: Disconnected, Status: "Disconnected"}
		if session, ok := sessions[path]; ok {
			profile = c.enrich(profile, session)
		}
		profiles = append(profiles, profile)
	}

	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].State == Connected && profiles[j].State != Connected {
			return true
		}
		if profiles[j].State == Connected && profiles[i].State != Connected {
			return false
		}
		return strings.ToLower(profiles[i].Name) < strings.ToLower(profiles[j].Name)
	})
	return profiles, nil
}

func (c *Client) Import(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("enter an .ovpn file")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if strings.ToLower(filepath.Ext(absPath)) != ".ovpn" {
		return errors.New("profile must have an .ovpn extension")
	}
	name := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
	output, err := exec.Command(
		"openvpn3", "config-import",
		"--config", absPath,
		"--name", name,
		"--persistent",
	).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("importing profile: %s", message)
	}
	return nil
}

func (c *Client) Remove(path dbus.ObjectPath) error {
	if !path.IsValid() {
		return errors.New("invalid profile")
	}
	err := c.conn.Object(configurationService, path).Call(configurationIface+".Remove", 0).Err
	return friendlyError("removing profile", err)
}

func (c *Client) BeginConnect(configPath dbus.ObjectPath) (ConnectResult, error) {
	if !configPath.IsValid() {
		return ConnectResult{}, errors.New("invalid profile")
	}
	var sessionPath dbus.ObjectPath
	err := c.conn.Object(sessionsService, sessionsPath).
		Call(sessionsIface+".NewTunnel", 0, configPath).
		Store(&sessionPath)
	if err != nil {
		return ConnectResult{}, friendlyError("creating VPN session", err)
	}

	result := ConnectResult{SessionPath: sessionPath}
	requests, readyErr := c.prepare(sessionPath)
	if len(requests) > 0 {
		result.Requests = requests
		return result, nil
	}
	if readyErr != nil {
		_ = c.Disconnect(sessionPath)
		return ConnectResult{}, readyErr
	}
	if err := c.connect(sessionPath); err != nil {
		_ = c.Disconnect(sessionPath)
		return ConnectResult{}, err
	}
	return result, nil
}

func (c *Client) ContinueConnect(sessionPath dbus.ObjectPath) (ConnectResult, error) {
	result := ConnectResult{SessionPath: sessionPath}
	requests, readyErr := c.prepare(sessionPath)
	if len(requests) > 0 {
		result.Requests = requests
		return result, nil
	}
	if readyErr != nil {
		return result, readyErr
	}
	return result, c.connect(sessionPath)
}

func (c *Client) Provide(sessionPath dbus.ObjectPath, request InputRequest, value string) error {
	err := c.conn.Object(sessionsService, sessionPath).
		Call(sessionsIface+".UserInputProvide", 0, request.Type, request.Group, request.ID, value).Err
	return friendlyError("providing authentication input", err)
}

func (c *Client) PendingInputs(sessionPath dbus.ObjectPath) ([]InputRequest, error) {
	obj := c.conn.Object(sessionsService, sessionPath)
	var groups []attentionTypeGroup
	if err := obj.Call(sessionsIface+".UserInputQueueGetTypeGroup", 0).Store(&groups); err != nil {
		return nil, friendlyError("fetching authentication requests", err)
	}

	var requests []InputRequest
	for _, group := range groups {
		// OPEN_URL is delivered as a status event; there is no value to type.
		if group.Type != 1 {
			continue
		}
		var ids []uint32
		if err := obj.Call(sessionsIface+".UserInputQueueCheck", 0, group.Type, group.Group).Store(&ids); err != nil {
			return nil, friendlyError("fetching authentication fields", err)
		}
		for _, id := range ids {
			var typ, grp, fetchedID uint32
			var name, description string
			var hidden bool
			err := obj.Call(sessionsIface+".UserInputQueueFetch", 0, group.Type, group.Group, id).
				Store(&typ, &grp, &fetchedID, &name, &description, &hidden)
			if err != nil {
				return nil, friendlyError("fetching authentication field", err)
			}
			requests = append(requests, InputRequest{
				Type: typ, Group: grp, ID: fetchedID, Name: name,
				Description: description, Hidden: hidden,
			})
		}
	}
	return requests, nil
}

func (c *Client) Disconnect(sessionPath dbus.ObjectPath) error {
	if !sessionPath.IsValid() {
		return errors.New("no active VPN session")
	}
	err := c.conn.Object(sessionsService, sessionPath).Call(sessionsIface+".Disconnect", 0).Err
	return friendlyError("disconnecting", err)
}

func (c *Client) prepare(sessionPath dbus.ObjectPath) ([]InputRequest, error) {
	readyErr := c.conn.Object(sessionsService, sessionPath).Call(sessionsIface+".Ready", 0).Err
	if readyErr == nil {
		return nil, nil
	}
	requests, err := c.PendingInputs(sessionPath)
	if err != nil {
		return nil, err
	}
	if len(requests) > 0 {
		return requests, nil
	}
	return nil, friendlyError("VPN is not ready", readyErr)
}

func (c *Client) connect(sessionPath dbus.ObjectPath) error {
	err := c.conn.Object(sessionsService, sessionPath).Call(sessionsIface+".Connect", 0).Err
	return friendlyError("connecting", err)
}

func (c *Client) sessionsByConfig() (map[dbus.ObjectPath]dbus.ObjectPath, error) {
	var paths []dbus.ObjectPath
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		err = c.conn.Object(sessionsService, sessionsPath).
			Call(sessionsIface+".FetchAvailableSessions", 0).
			Store(&paths)
		if err == nil || !objectIsStarting(err) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		return nil, friendlyError("fetching sessions", err)
	}

	result := make(map[dbus.ObjectPath]dbus.ObjectPath, len(paths))
	for _, sessionPath := range paths {
		obj := c.conn.Object(sessionsService, sessionPath)
		var configPath dbus.ObjectPath
		if err := getProperty(obj, sessionsIface, "config_path", &configPath); err != nil {
			continue
		}
		if previous, ok := result[configPath]; ok {
			var previousStatus statusTuple
			_ = getProperty(c.conn.Object(sessionsService, previous), sessionsIface, "status", &previousStatus)
			var candidateStatus statusTuple
			_ = getProperty(obj, sessionsIface, "status", &candidateStatus)
			if previousStatus.Minor == statusConnected || candidateStatus.Minor != statusConnected {
				continue
			}
		}
		result[configPath] = sessionPath
	}
	return result, nil
}

func (c *Client) enrich(profile Profile, sessionPath dbus.ObjectPath) Profile {
	obj := c.conn.Object(sessionsService, sessionPath)
	profile.SessionPath = sessionPath

	var status statusTuple
	if err := getProperty(obj, sessionsIface, "status", &status); err == nil {
		profile.State = stateFromStatus(status)
		profile.Status = statusText(status)
		if status.Major == statusMajorSession && status.Minor == statusAuthURL && validWebURL(status.Message) {
			profile.AuthURL = status.Message
		}
	}
	_ = getProperty(obj, sessionsIface, "device_name", &profile.Device)
	var created uint64
	if err := getProperty(obj, sessionsIface, "session_created", &created); err == nil {
		profile.CreatedAt = time.Unix(int64(created), 0)
	}
	var connected connectedTo
	if err := getProperty(obj, sessionsIface, "connected_to", &connected); err == nil {
		profile.Protocol = strings.ToUpper(connected.Protocol)
		profile.ServerIP = connected.ServerIP
		profile.ServerPort = connected.ServerPort
	}
	var stats map[string]int64
	if err := getProperty(obj, sessionsIface, "statistics", &stats); err == nil {
		// Socket counters match user-facing receive/send direction. If they are
		// unavailable, TUN output is received traffic and TUN input is sent.
		profile.BytesIn = positiveStat(stats, "BYTES_IN", "TUN_BYTES_OUT")
		profile.BytesOut = positiveStat(stats, "BYTES_OUT", "TUN_BYTES_IN")
	}
	return profile
}

type attentionTypeGroup struct {
	Type  uint32
	Group uint32
}

type statusTuple struct {
	Major   uint32
	Minor   uint32
	Message string
}

type connectedTo struct {
	Protocol   string
	ServerIP   string
	ServerPort uint32
}

const (
	statusMajorSession = 3
	statusConnected    = 7
	statusAuthURL      = 22
)

func stateFromStatus(status statusTuple) State {
	switch status.Minor {
	case 4, 5, 6, 12, 15, 17, 20, 21:
		return Connecting
	case statusConnected:
		return Connected
	case 8:
		return Disconnecting
	case 13, 14:
		return Paused
	case statusAuthURL:
		return WaitingForBrowser
	case 1, 3, 10, 11, 28, 29:
		return Failed
	default:
		return Disconnected
	}
}

func statusText(status statusTuple) string {
	if status.Major == statusMajorSession && status.Minor == statusAuthURL {
		return "Sign in with browser"
	}
	switch status.Minor {
	case 4, 20, 21:
		return "Authentication required"
	case 5:
		return "Preparing"
	case 6:
		return "Connecting…"
	case statusConnected:
		return "Connected"
	case 8:
		return "Disconnecting…"
	case 9, 16, 18, 19:
		return "Disconnected"
	case 10:
		return "Connection failed"
	case 11:
		return "Authentication failed"
	case 12:
		return "Reconnecting…"
	case 13, 14:
		return "Paused"
	case 15:
		return "Resuming…"
	case 17:
		return "Starting session…"
	}
	if strings.TrimSpace(status.Message) != "" {
		return status.Message
	}
	return "Disconnected"
}

func positiveStat(stats map[string]int64, preferred, fallback string) uint64 {
	value, ok := stats[preferred]
	if !ok {
		value = stats[fallback]
	}
	if value < 0 {
		return 0
	}
	return uint64(value)
}

func getProperty(obj dbus.BusObject, iface, name string, destination any) error {
	variant, err := obj.GetProperty(iface + "." + name)
	if err != nil {
		return err
	}
	return dbus.Store([]any{variant.Value()}, destination)
}

func friendlyError(action string, err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if idx := strings.LastIndex(message, ":"); idx >= 0 && idx+1 < len(message) {
		message = strings.TrimSpace(message[idx+1:])
	}
	return fmt.Errorf("%s: %s", action, message)
}

func objectIsStarting(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "UnknownObject") ||
		strings.Contains(message, "Object does not exist at path")
}

func validWebURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}
