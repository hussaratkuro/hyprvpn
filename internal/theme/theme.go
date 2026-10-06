// Package theme maps HyDE Wallbash colors to a shared GUI palette.
package theme

import (
	"bufio"
	"encoding/hex"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const refreshInterval = 2 * time.Second

type Palette struct {
	Base, Mantle, Crust, Surface0, Surface1, Surface2, Overlay0, Overlay1, Overlay2, Subtext0, Subtext1, Text, Lavender, Blue, Sapphire, Sky, Teal, Green, Yellow, Peach, Red, Mauve, Pink, Rosewater, OnAccent color.NRGBA
}

// Watch polls the wallbash file and calls onChange from a goroutine whenever
// the palette differs from the last one it reported.
func Watch(onChange func(Palette)) {
	go func() {
		last := Current()
		for range time.Tick(refreshInterval) {
			if p := Current(); p != last {
				last = p
				onChange(p)
			}
		}
	}()
}

func Current() Palette {
	f := CatppuccinMocha()
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TUI_THEME")), "catppuccin") {
		return f
	}
	v, ok := readColors(themePath())
	if !ok || v["wallbash_pry1"] == "" || v["wallbash_txt1"] == "" {
		return f
	}
	p := Palette{
		Base: pick(v, "wallbash_pry1", f.Base), Mantle: pick(v, "wallbash_pry2", f.Mantle), Crust: pick(v, "wallbash_1xa1", f.Crust), Surface0: pick(v, "wallbash_1xa2", f.Surface0), Surface1: pick(v, "wallbash_1xa3", f.Surface1), Surface2: pick(v, "wallbash_1xa4", f.Surface2),
		Overlay0: pick(v, "wallbash_1xa5", f.Overlay0), Overlay1: pick(v, "wallbash_1xa6", f.Overlay1), Overlay2: pick(v, "wallbash_1xa7", f.Overlay2), Subtext0: pick(v, "wallbash_1xa8", f.Subtext0), Subtext1: pick(v, "wallbash_1xa9", f.Subtext1), Text: pick(v, "wallbash_txt1", f.Text),
		Lavender: pick(v, "wallbash_3xa8", f.Lavender), Blue: pick(v, "wallbash_3xa7", f.Blue), Sapphire: pick(v, "wallbash_3xa8", f.Sapphire), Sky: pick(v, "wallbash_2xa8", f.Sky), Teal: pick(v, "wallbash_2xa7", f.Teal), Green: pick(v, "wallbash_2xa9", f.Green), Yellow: pick(v, "wallbash_1xa8", f.Yellow), Peach: pick(v, "wallbash_1xa9", f.Peach), Red: pick(v, "wallbash_4xa8", f.Red), Mauve: pick(v, "wallbash_3xa8", f.Mauve), Pink: pick(v, "wallbash_2xa8", f.Pink), Rosewater: pick(v, "wallbash_4xa9", f.Rosewater),
	}
	p.OnAccent = contrasting(p.Mauve)
	return p
}

func CatppuccinMocha() Palette {
	return Palette{Base: Hex("#1e1e2e"), Mantle: Hex("#181825"), Crust: Hex("#11111b"), Surface0: Hex("#313244"), Surface1: Hex("#45475a"), Surface2: Hex("#585b70"), Overlay0: Hex("#6c7086"), Overlay1: Hex("#7f849c"), Overlay2: Hex("#9399b2"), Subtext0: Hex("#a6adc8"), Subtext1: Hex("#bac2de"), Text: Hex("#cdd6f4"), Lavender: Hex("#b4befe"), Blue: Hex("#89b4fa"), Sapphire: Hex("#74c7ec"), Sky: Hex("#89dceb"), Teal: Hex("#94e2d5"), Green: Hex("#a6e3a1"), Yellow: Hex("#f9e2af"), Peach: Hex("#fab387"), Red: Hex("#f38ba8"), Mauve: Hex("#cba6f7"), Pink: Hex("#f5c2e7"), Rosewater: Hex("#f5e0dc"), OnAccent: Hex("#1e1e2e")}
}

// Hex parses "#rrggbb"; invalid input yields opaque black.
func Hex(s string) color.NRGBA {
	b, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(s), "#"))
	if err != nil || len(b) != 3 {
		return color.NRGBA{A: 0xff}
	}
	return color.NRGBA{R: b[0], G: b[1], B: b[2], A: 0xff}
}

func themePath() string {
	if p := strings.TrimSpace(os.Getenv("TUI_THEME_FILE")); p != "" {
		return p
	}
	c := strings.TrimSpace(os.Getenv("XDG_CACHE_HOME"))
	if c == "" {
		if h, e := os.UserHomeDir(); e == nil {
			c = filepath.Join(h, ".cache")
		}
	}
	return filepath.Join(c, "hyde", "wallbash", "shell-colors")
}

func readColors(path string) (map[string]string, bool) {
	f, e := os.Open(path)
	if e != nil {
		return nil, false
	}
	defer f.Close()
	v := make(map[string]string)
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, r, ok := strings.Cut(line, "=")
		if !ok || strings.HasSuffix(k, "_rgba") {
			continue
		}
		fields := strings.Fields(r)
		if len(fields) == 0 {
			continue
		}
		r = strings.Trim(fields[0], "'\"")
		if n, ok := normalizeHex(r); ok {
			v[strings.TrimSpace(k)] = n
		}
	}
	return v, s.Err() == nil
}

func normalizeHex(v string) (string, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "#")
	if len(v) != 6 {
		return "", false
	}
	if _, e := hex.DecodeString(v); e != nil {
		return "", false
	}
	return "#" + strings.ToUpper(v), true
}

func pick(v map[string]string, k string, f color.NRGBA) color.NRGBA {
	if x := v[k]; x != "" {
		return Hex(x)
	}
	return f
}

func contrasting(c color.NRGBA) color.NRGBA {
	if (299*int(c.R)+587*int(c.G)+114*int(c.B))/1000 >= 150 {
		return Hex("#111111")
	}
	return Hex("#ffffff")
}
