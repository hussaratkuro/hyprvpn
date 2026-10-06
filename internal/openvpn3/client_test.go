package openvpn3

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestDBusTupleDecoding(t *testing.T) {
	want := statusTuple{Major: 3, Minor: 22, Message: "https://vpn.example.test/login"}
	variant := dbus.MakeVariant(want)
	var got statusTuple
	if err := dbus.Store([]any{variant.Value()}, &got); err != nil {
		t.Fatalf("store D-Bus status tuple: %v", err)
	}
	if got != want {
		t.Fatalf("decoded tuple = %#v, want %#v", got, want)
	}
}

func TestStateFromStatus(t *testing.T) {
	tests := []struct {
		minor uint32
		want  State
	}{
		{4, Connecting},
		{6, Connecting},
		{7, Connected},
		{9, Disconnected},
		{10, Failed},
		{14, Paused},
		{22, WaitingForBrowser},
	}
	for _, test := range tests {
		if got := stateFromStatus(statusTuple{Minor: test.minor}); got != test.want {
			t.Errorf("minor %d: got %v, want %v", test.minor, got, test.want)
		}
	}
}

func TestStatusTextIsEnglish(t *testing.T) {
	tests := []struct {
		minor uint32
		want  string
	}{
		{4, "Authentication required"},
		{6, "Connecting…"},
		{statusConnected, "Connected"},
		{8, "Disconnecting…"},
		{10, "Connection failed"},
		{statusAuthURL, "Sign in with browser"},
	}
	for _, test := range tests {
		status := statusTuple{Major: statusMajorSession, Minor: test.minor}
		if got := statusText(status); got != test.want {
			t.Errorf("minor %d: got %q, want %q", test.minor, got, test.want)
		}
	}
}

func TestValidWebURL(t *testing.T) {
	for _, raw := range []string{"https://vpn.example.test/oauth", "http://127.0.0.1:8080/login"} {
		if !validWebURL(raw) {
			t.Errorf("expected valid URL: %q", raw)
		}
	}
	for _, raw := range []string{"javascript:alert(1)", "file:///etc/passwd", "not a url", "https:///missing-host"} {
		if validWebURL(raw) {
			t.Errorf("expected rejected URL: %q", raw)
		}
	}
}

func TestPositiveStatPrefersSocketBytes(t *testing.T) {
	stats := map[string]int64{"TUN_BYTES_OUT": 42, "BYTES_IN": 99}
	if got := positiveStat(stats, "BYTES_IN", "TUN_BYTES_OUT"); got != 99 {
		t.Fatalf("got %d, want 99", got)
	}
	if got := positiveStat(map[string]int64{"TUN_BYTES_OUT": 42}, "BYTES_IN", "TUN_BYTES_OUT"); got != 42 {
		t.Fatalf("fallback got %d, want 42", got)
	}
}
