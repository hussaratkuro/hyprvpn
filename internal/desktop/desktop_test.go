package desktop

import "testing"

func TestOpenBrowserRejectsUnsafeURLs(t *testing.T) {
	for _, rawURL := range []string{"javascript:alert(1)", "file:///etc/passwd", "https:///missing-host", "not a url"} {
		if err := OpenBrowser(rawURL); err == nil {
			t.Errorf("OpenBrowser(%q) should reject URL", rawURL)
		}
	}
}
