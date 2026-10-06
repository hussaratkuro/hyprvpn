package ui

import (
	"errors"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/hussaratkuro/hyprvpn/internal/openvpn3"
)

// DemoBackend is a fake OpenVPN 3 client for previewing the UI without a
// VPN (HYPRVPN_DEMO=1). It never touches D-Bus.
type DemoBackend struct {
	profiles []openvpn3.Profile
	started  time.Time
}

func NewDemoBackend() *DemoBackend {
	return &DemoBackend{started: time.Now(), profiles: []openvpn3.Profile{
		{Path: "/demo/office", Name: "Office", Persistent: true, SessionPath: "/demo/session/1", State: openvpn3.Connected, Status: "Connected", Device: "tun0", Protocol: "UDP", ServerIP: "203.0.113.10", ServerPort: 1194, CreatedAt: time.Now().Add(-37 * time.Minute)},
		{Path: "/demo/home", Name: "Home lab", Persistent: true, State: openvpn3.Disconnected, Status: "Disconnected"},
		{Path: "/demo/oauth", Name: "Cloud (OAuth)", Persistent: true, State: openvpn3.Disconnected, Status: "Disconnected"},
	}}
}

func (d *DemoBackend) Profiles() ([]openvpn3.Profile, error) {
	out := make([]openvpn3.Profile, len(d.profiles))
	copy(out, d.profiles)
	elapsed := time.Since(d.started).Seconds()
	for i := range out {
		if out[i].State == openvpn3.Connected {
			out[i].BytesIn = uint64(elapsed*180000) + uint64(elapsed*elapsed*900)
			out[i].BytesOut = uint64(elapsed * 42000)
		}
	}
	return out, nil
}

func (d *DemoBackend) Import(string) error          { return errors.New("demo mode cannot import") }
func (d *DemoBackend) Remove(dbus.ObjectPath) error { return nil }

func (d *DemoBackend) BeginConnect(path dbus.ObjectPath) (openvpn3.ConnectResult, error) {
	for i := range d.profiles {
		if d.profiles[i].Path != path {
			continue
		}
		if d.profiles[i].Name == "Home lab" {
			return openvpn3.ConnectResult{SessionPath: "/demo/session/2", Requests: []openvpn3.InputRequest{{Name: "username", Description: "Auth User name"}, {Name: "password", Description: "Auth Password", Hidden: true}}}, nil
		}
		d.profiles[i].SessionPath, d.profiles[i].State, d.profiles[i].Status = "/demo/session/3", openvpn3.WaitingForBrowser, "Waiting for browser sign-in"
		d.profiles[i].AuthURL = "https://auth.example.com/oauth/authorize?state=demo"
	}
	return openvpn3.ConnectResult{SessionPath: "/demo/session/3"}, nil
}

func (d *DemoBackend) ContinueConnect(dbus.ObjectPath) (openvpn3.ConnectResult, error) {
	for i := range d.profiles {
		if d.profiles[i].Name == "Home lab" {
			d.profiles[i].SessionPath, d.profiles[i].State, d.profiles[i].Status = "/demo/session/2", openvpn3.Connected, "Connected"
			d.profiles[i].Device, d.profiles[i].Protocol, d.profiles[i].ServerIP, d.profiles[i].ServerPort, d.profiles[i].CreatedAt = "tun1", "TCP", "198.51.100.7", 443, time.Now()
		}
	}
	return openvpn3.ConnectResult{}, nil
}

func (d *DemoBackend) Provide(dbus.ObjectPath, openvpn3.InputRequest, string) error { return nil }
func (d *DemoBackend) PendingInputs(dbus.ObjectPath) ([]openvpn3.InputRequest, error) {
	return nil, nil
}

func (d *DemoBackend) Disconnect(session dbus.ObjectPath) error {
	for i := range d.profiles {
		if d.profiles[i].SessionPath == session {
			d.profiles[i].SessionPath, d.profiles[i].State, d.profiles[i].Status, d.profiles[i].AuthURL = "", openvpn3.Disconnected, "Disconnected", ""
		}
	}
	return nil
}
