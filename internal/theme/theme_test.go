package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentReadsWallbashAndFallsBack(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell-colors")
	os.WriteFile(path, []byte("wallbash_pry1='#101010'\nwallbash_txt1=\"#EEEEEE\"\nwallbash_pry1_rgba=x\n"), 0o644)
	t.Setenv("TUI_THEME_FILE", path)
	t.Setenv("TUI_THEME", "")
	p := Current()
	if p.Base != Hex("#101010") || p.Text != Hex("#eeeeee") || p.Mauve != CatppuccinMocha().Mauve {
		t.Fatalf("unexpected palette %+v", p)
	}
	t.Setenv("TUI_THEME_FILE", filepath.Join(dir, "missing"))
	if Current() != CatppuccinMocha() {
		t.Fatal("missing file should fall back to Catppuccin")
	}
}
