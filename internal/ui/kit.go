// Shared drawing kit for the hypr* Gio apps (copied from hyprvault).
package ui

import (
	"image"
	"image/color"
	"os"
	"path/filepath"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

// Nerd Font glyphs (Font Awesome range); the Go fallback font shows a box
// when JetBrainsMono Nerd Font is not installed.
const (
	icoPower    = "\uF011"
	icoDown     = "\uF019"
	icoUp       = "\uF093"
	icoPlus     = "\uF067"
	icoDelete   = "\uF1F8"
	icoRefresh  = "\uF021"
	icoGlobe    = "\uF0AC"
	icoKey      = "\uF084"
	icoLock     = "\uF023"
	icoUnlock   = "\uF09C"
	icoCheck    = "\uF00C"
	icoBack     = "\uF060"
	icoLink     = "\uF0C1"
	icoServer   = "\uF233"
	icoClock    = "\uF017"
	icoShield   = "\uF132"
	icoDot      = "\uF111"
	icoExternal = "\uF08E"
	icoTimes    = "\uF00D"
	icoFile     = "\uF15B"
	icoInfo     = "\uF05A"
	icoWarn     = "\uF071"
)

const (
	radius   = unit.Dp(10)
	nerdFont = "JetBrainsMono Nerd Font Propo"
)

// loadFonts prefers the user's JetBrainsMono Nerd Font (HyDE default) and
// always appends the Go fonts as fallback.
func loadFonts() []font.FontFace {
	var faces []font.FontFace
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".local", "share", "fonts", "JetBrainsMono")
	for _, v := range []struct {
		file   string
		weight font.Weight
	}{{"Regular", font.Normal}, {"Medium", font.Medium}, {"SemiBold", font.SemiBold}, {"Bold", font.Bold}} {
		data, err := os.ReadFile(filepath.Join(dir, "JetBrainsMonoNerdFontPropo-"+v.file+".ttf"))
		if err != nil {
			continue
		}
		face, err := opentype.Parse(data)
		if err != nil {
			continue
		}
		faces = append(faces, font.FontFace{Font: font.Font{Typeface: nerdFont, Weight: v.weight}, Face: face})
	}
	// Symbols the Nerd font lacks (arrows, box drawing, ⏎ ◷ ★ …) fall back
	// to Noto so nothing renders as a tofu box.
	for _, f := range []string{"/usr/share/fonts/noto/NotoSansSymbols-Regular.ttf", "/usr/share/fonts/noto/NotoSansSymbols2-Regular.ttf", "/usr/share/fonts/noto/NotoSansMono-Regular.ttf"} {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if face, err := opentype.Parse(data); err == nil {
			faces = append(faces, font.FontFace{Font: font.Font{Typeface: "Noto Fallback"}, Face: face})
		}
	}
	return append(faces, gofont.Collection()...)
}

// ---- drawing helpers ----

func alpha(c color.NRGBA, a uint8) color.NRGBA { c.A = a; return c }

// box draws a rounded, bordered container around w.
func (a *App) box(gtx C, bg, border color.NRGBA, r unit.Dp, inset layout.Inset, w layout.Widget) D {
	macro := op.Record(gtx.Ops)
	dims := inset.Layout(gtx, w)
	call := macro.Stop()
	// A radius above half the box makes Gio's RRect fold in on itself.
	rr := clip.UniformRRect(image.Rectangle{Max: dims.Size}, min(gtx.Dp(r), dims.Size.X/2, dims.Size.Y/2))
	paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
	if border.A != 0 {
		paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
	}
	call.Add(gtx.Ops)
	return dims
}

// panel is the standard content surface.
func (a *App) panel(w layout.Widget) layout.Widget {
	return func(gtx C) D {
		return a.box(gtx, alpha(a.pal.Mantle, 0xF2), a.pal.Surface0, radius, layout.UniformInset(unit.Dp(22)), w)
	}
}

func (a *App) narrow(maxW unit.Dp, w layout.Widget) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(maxW))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return w(gtx)
	}
}

func (a *App) column(gtx C, ws ...layout.Widget) D {
	children := make([]layout.FlexChild, 0, len(ws)*2)
	for i, w := range ws {
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout))
		}
		children = append(children, layout.Rigid(w))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) row(ws ...layout.Widget) layout.Widget {
	return func(gtx C) D {
		children := make([]layout.FlexChild, 0, len(ws)*2)
		for i, w := range ws {
			if i > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout))
			}
			children = append(children, layout.Rigid(w))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	}
}

func (a *App) heading(s string) layout.Widget {
	return func(gtx C) D {
		l := material.Label(a.th, unit.Sp(22), s)
		l.Color = a.pal.Text
		l.Font.Weight = font.SemiBold
		return l.Layout(gtx)
	}
}

func (a *App) text(s string, c color.NRGBA) layout.Widget {
	return func(gtx C) D {
		l := material.Label(a.th, unit.Sp(14), s)
		l.Color = c
		return l.Layout(gtx)
	}
}

func (a *App) small(s string, c color.NRGBA) layout.Widget {
	return func(gtx C) D {
		l := material.Label(a.th, unit.Sp(12), s)
		l.Color = c
		return l.Layout(gtx)
	}
}

func (a *App) mono(s string, c color.NRGBA, size unit.Sp) layout.Widget {
	return func(gtx C) D {
		l := material.Label(a.th, size, s)
		l.Color = c
		l.Font.Weight = font.Medium
		return l.Layout(gtx)
	}
}

func (a *App) icon(s string, c color.NRGBA, size unit.Sp) layout.Widget {
	return func(gtx C) D {
		l := material.Label(a.th, size, s)
		l.Color = c
		return l.Layout(gtx)
	}
}

// iconDisc draws a glyph on a tinted circle.
func (a *App) iconDisc(ico string, accent color.NRGBA, size unit.Dp) layout.Widget {
	return func(gtx C) D {
		d := gtx.Dp(size)
		paint.FillShape(gtx.Ops, alpha(accent, 0x30), clip.Ellipse{Max: image.Pt(d, d)}.Op(gtx.Ops))
		gtx.Constraints = layout.Exact(image.Pt(d, d))
		return layout.Center.Layout(gtx, a.icon(ico, accent, unit.Sp(float32(size)*0.5)))
	}
}

// keycap renders a keyboard hint like the ones in HyDE's rofi menus.
func (a *App) keycap(s string) layout.Widget {
	return func(gtx C) D {
		return a.box(gtx, a.pal.Surface1, color.NRGBA{}, unit.Dp(5), layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(7), Right: unit.Dp(7)}, a.small(s, a.pal.Subtext1))
	}
}

// status renders the manager's error or success line as a toast strip.
func (a *App) status(err, ok string) layout.Widget {
	return func(gtx C) D {
		msg, c, ico := ok, a.pal.Green, icoCheck
		if err != "" {
			msg, c, ico = err, a.pal.Red, "!"
		}
		if msg == "" {
			return D{}
		}
		return a.box(gtx, alpha(c, 0x22), color.NRGBA{}, unit.Dp(8), layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(12)},
			a.row(a.icon(ico, c, unit.Sp(14)), a.text(msg, c)))
	}
}

// field is a labeled single-line editor.
func (a *App) field(label string, ed *widget.Editor, hint string, focused bool) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(a.small(label, a.pal.Subtext0)),
			layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
			layout.Rigid(a.editor(ed, hint, focused)),
		)
	}
}

func (a *App) editor(ed *widget.Editor, hint string, focused bool) layout.Widget {
	return func(gtx C) D {
		border := a.pal.Surface1
		if focused {
			border = a.pal.Mauve
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return a.box(gtx, a.pal.Surface0, border, unit.Dp(8), layout.Inset{Top: unit.Dp(9), Bottom: unit.Dp(9), Left: unit.Dp(12), Right: unit.Dp(12)}, func(gtx C) D {
			e := material.Editor(a.th, ed, hint)
			e.Color = a.pal.Text
			e.HintColor = a.pal.Overlay0
			e.SelectionColor = alpha(a.pal.Mauve, 0x66)
			e.TextSize = unit.Sp(14)
			return e.Layout(gtx)
		})
	}
}

type btnKind int

const (
	btnPrimary btnKind = iota
	btnSecondary
	btnDanger
)

// btn is a pill button with an optional Nerd Font icon and hover state.
func (a *App) btn(c *widget.Clickable, ico, label string, kind btnKind) layout.Widget {
	return func(gtx C) D {
		bg, fg := a.pal.Surface1, a.pal.Text
		switch kind {
		case btnPrimary:
			bg, fg = a.pal.Mauve, a.pal.OnAccent
		case btnDanger:
			bg, fg = alpha(a.pal.Red, 0x33), a.pal.Red
		}
		if c.Hovered() {
			bg = mix(bg, a.pal.Text, 0.10)
		}
		return c.Layout(gtx, func(gtx C) D {
			return a.box(gtx, bg, color.NRGBA{}, unit.Dp(8), layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(14), Right: unit.Dp(14)}, func(gtx C) D {
				pointer.CursorPointer.Add(gtx.Ops)
				ws := []layout.Widget{}
				if ico != "" {
					ws = append(ws, a.icon(ico, fg, unit.Sp(14)))
				}
				if label != "" {
					ws = append(ws, a.mono(label, fg, unit.Sp(13)))
				}
				return a.row(ws...)(gtx)
			})
		})
	}
}

func (a *App) iconBtn(c *widget.Clickable, ico string) layout.Widget {
	return func(gtx C) D {
		bg := color.NRGBA{}
		if c.Hovered() {
			bg = a.pal.Surface0
		}
		return c.Layout(gtx, func(gtx C) D {
			return a.box(gtx, bg, color.NRGBA{}, unit.Dp(8), layout.UniformInset(unit.Dp(6)), func(gtx C) D {
				pointer.CursorPointer.Add(gtx.Ops)
				return a.icon(ico, a.pal.Subtext1, unit.Sp(18))(gtx)
			})
		})
	}
}

func mix(a, b color.NRGBA, t float32) color.NRGBA {
	l := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*t) }
	return color.NRGBA{R: l(a.R, b.R), G: l(a.G, b.G), B: l(a.B, b.B), A: a.A}
}

func newEditor(single, submit bool, mask rune) widget.Editor {
	return widget.Editor{SingleLine: single, Submit: submit, Mask: mask}
}
