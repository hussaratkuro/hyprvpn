package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/hussaratkuro/hyprvpn/internal/openvpn3"
)

func (a *App) layoutDashboard(gtx C) D {
	if a.mode == modeDashboard && !a.busy {
		if a.importBtn.Clicked(gtx) {
			a.openImport()
		}
		if a.refreshBtn.Clicked(gtx) {
			a.refresh()
		}
		if a.powerBtn.Clicked(gtx) {
			a.togglePower()
		}
		if a.browserBtn.Clicked(gtx) {
			if p, ok := a.selectedProfile(); ok && p.AuthURL != "" {
				a.openBrowser(p.AuthURL)
			}
		}
		if a.removeBtn.Clicked(gtx) {
			if p, ok := a.selectedProfile(); ok && p.SessionPath == "" {
				a.mode = modeDelete
			}
		}
		for i := range a.profiles {
			if a.rowClicks[i].Clicked(gtx) {
				a.selected = i
				a.notice, a.errorText = "", ""
			}
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(a.header),
		layout.Flexed(1, func(gtx C) D {
			inset := layout.Inset{Left: unit.Dp(24), Right: unit.Dp(24), Bottom: unit.Dp(20)}
			if gtx.Constraints.Max.X < gtx.Dp(760) {
				return inset.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D { gtx.Constraints.Max.Y = gtx.Dp(190); return a.sidebar(gtx) }),
						layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
						layout.Flexed(1, a.detail),
					)
				})
			}
			return inset.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D { gtx.Constraints.Max.X = gtx.Dp(270); return a.sidebar(gtx) }),
					layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
					layout.Flexed(1, a.detail),
				)
			})
		}),
	)
}

func (a *App) header(gtx C) D {
	return layout.Inset{Top: unit.Dp(18), Bottom: unit.Dp(16), Left: unit.Dp(24), Right: unit.Dp(24)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(a.iconDisc(icoShield, a.pal.Mauve, 30)),
			layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
			layout.Rigid(func(gtx C) D {
				l := material.Label(a.th, unit.Sp(17), "hyprvpn")
				l.Color = a.pal.Text
				l.Font.Weight = font.Bold
				return l.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
			layout.Rigid(a.small("OpenVPN 3 · "+a.version, a.pal.Overlay0)),
			layout.Flexed(1, layout.Spacer{}.Layout),
			layout.Rigid(func(gtx C) D {
				if !a.loading && !a.busy {
					return D{}
				}
				return a.row(func(gtx C) D { gtx.Constraints.Max.X = gtx.Dp(16); return material.Loader(a.th).Layout(gtx) }, a.small("working…", a.pal.Peach))(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
			layout.Rigid(a.paletteDots),
		)
	})
}

func (a *App) paletteDots(gtx C) D {
	cols := []color.NRGBA{a.pal.Mauve, a.pal.Lavender, a.pal.Sapphire, a.pal.Green, a.pal.Peach, a.pal.Red}
	children := make([]layout.FlexChild, 0, len(cols)*2)
	for i, c := range cols {
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout))
		}
		c := c
		children = append(children, layout.Rigid(func(gtx C) D {
			d := gtx.Dp(8)
			paint.FillShape(gtx.Ops, c, clip.Ellipse{Max: image.Pt(d, d)}.Op(gtx.Ops))
			return D{Size: image.Pt(d, d)}
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// ---- sidebar ----

func (a *App) sidebar(gtx C) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	return a.box(gtx, alpha(a.pal.Mantle, 0xF2), a.pal.Surface0, radius, layout.UniformInset(unit.Dp(12)), func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(2), Bottom: unit.Dp(10)}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, a.small("PROFILES", a.pal.Subtext0)),
						layout.Rigid(a.iconBtn(&a.refreshBtn, icoRefresh)),
						layout.Rigid(a.iconBtn(&a.importBtn, icoPlus)),
					)
				})
			}),
			layout.Flexed(1, a.profileList),
		)
	})
}

func (a *App) profileList(gtx C) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	if len(a.profiles) == 0 {
		msg := "No profiles yet.\nImport an .ovpn file with the + button or i."
		if a.loadError {
			msg = "OpenVPN 3 is not reachable."
		}
		return layout.Center.Layout(gtx, a.small(msg, a.pal.Overlay1))
	}
	ls := material.List(a.th, &a.list)
	ls.Indicator.Color = a.pal.Surface2
	ls.Track.Color = alpha(a.pal.Surface0, 0)
	rowH := gtx.Dp(44)
	return ls.Layout(gtx, len(a.profiles), func(gtx C, i int) D {
		p := a.profiles[i]
		c := &a.rowClicks[i]
		return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx C) D {
			return c.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, rowH)
				size := gtx.Constraints.Min
				switch {
				case i == a.selected:
					paint.FillShape(gtx.Ops, a.pal.Surface0, clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(8)).Op(gtx.Ops))
					paint.FillShape(gtx.Ops, a.pal.Mauve, clip.UniformRRect(image.Rectangle{Min: image.Pt(0, gtx.Dp(10)), Max: image.Pt(gtx.Dp(3), rowH-gtx.Dp(10))}, gtx.Dp(2)).Op(gtx.Ops))
				case c.Hovered():
					paint.FillShape(gtx.Ops, alpha(a.pal.Surface0, 0x80), clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(8)).Op(gtx.Ops))
				}
				pointer.CursorPointer.Add(gtx.Ops)
				return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(10)}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(a.icon(icoDot, a.stateColor(p.State), unit.Sp(9))),
						layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
						layout.Flexed(1, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(a.text(p.Name, a.pal.Text)),
								layout.Rigid(a.small(p.Status, a.pal.Overlay1)),
							)
						}),
					)
				})
			})
		})
	})
}

// ---- detail ----

func (a *App) detail(gtx C) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	return a.box(gtx, alpha(a.pal.Mantle, 0xF2), a.pal.Surface0, radius, layout.UniformInset(unit.Dp(22)), func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		profile, ok := a.selectedProfile()
		if !ok {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Center.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(a.icon(icoShield, a.pal.Surface2, unit.Sp(40))),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(a.text("No VPN profile to display", a.pal.Overlay1)),
					layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
					layout.Rigid(a.status(a.errorText, a.notice)),
				)
			})
		}
		state := a.stateColor(profile.State)
		h := a.history[profile.Path]
		if h == nil {
			h = &trafficHistory{}
		}
		ws := []layout.Widget{
			func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, a.heading(profile.Name)),
					layout.Rigid(a.pill(profile.Status, state)),
				)
			},
			a.powerButton(profile, state),
			func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, a.trafficCard(icoDown, "RECEIVED", profile.BytesIn, h.rateIn, h.inSamples, a.pal.Sapphire)),
					layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
					layout.Flexed(1, a.trafficCard(icoUp, "SENT", profile.BytesOut, h.rateOut, h.outSamples, a.pal.Lavender)),
				)
			},
			a.connectionDetails(profile),
		}
		if profile.AuthURL != "" {
			ws = append(ws, a.oauthBox(profile.AuthURL))
		}
		if a.errorText != "" {
			ws = append(ws, a.status(a.errorText, ""))
		} else if a.notice != "" {
			ws = append(ws, a.status("", a.notice))
		}
		ws = append(ws, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, a.small("↑↓ profile · enter connect · o browser · i import · del remove · r refresh · q quit", a.pal.Overlay0)),
				layout.Rigid(func(gtx C) D {
					if profile.SessionPath != "" {
						return D{}
					}
					return a.btn(&a.removeBtn, icoDelete, "Remove", btnDanger)(gtx)
				}),
			)
		})
		return a.column(gtx, ws...)
	})
}

func (a *App) pill(s string, c color.NRGBA) layout.Widget {
	return func(gtx C) D {
		return a.box(gtx, alpha(c, 0x2A), color.NRGBA{}, unit.Dp(99), layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(10), Right: unit.Dp(10)}, a.mono(s, c, unit.Sp(12)))
	}
}

// powerButton is the big round connect/disconnect control.
func (a *App) powerButton(p openvpn3.Profile, state color.NRGBA) layout.Widget {
	return func(gtx C) D {
		label := "CONNECT"
		if p.SessionPath.IsValid() {
			label = "DISCONNECT"
		}
		return layout.Center.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return a.powerBtn.Layout(gtx, func(gtx C) D {
						d := gtx.Dp(96)
						fill := alpha(state, 0x22)
						if a.powerBtn.Hovered() && !a.busy {
							fill = alpha(state, 0x40)
						}
						paint.FillShape(gtx.Ops, fill, clip.Ellipse{Max: image.Pt(d, d)}.Op(gtx.Ops))
						paint.FillShape(gtx.Ops, state, clip.Stroke{Path: clip.Ellipse{Max: image.Pt(d, d)}.Path(gtx.Ops), Width: float32(gtx.Dp(2))}.Op())
						pointer.CursorPointer.Add(gtx.Ops)
						gtx.Constraints = layout.Exact(image.Pt(d, d))
						return layout.Center.Layout(gtx, a.icon(icoPower, state, unit.Sp(38)))
					})
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(a.mono(label, a.pal.Subtext0, unit.Sp(12))),
			)
		})
	}
}

func (a *App) trafficCard(ico, label string, total uint64, rate float64, samples []float64, c color.NRGBA) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return a.box(gtx, a.pal.Surface0, a.pal.Surface1, unit.Dp(8), layout.UniformInset(unit.Dp(14)), func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(a.row(a.icon(ico, c, unit.Sp(13)), a.mono(label, c, unit.Sp(11)))),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx C) D {
					l := material.Label(a.th, unit.Sp(20), formatBytes(float64(total)))
					l.Color = a.pal.Text
					l.Font.Weight = font.SemiBold
					return l.Layout(gtx)
				}),
				layout.Rigid(a.small(formatBytes(rate)+"/s", a.pal.Subtext0)),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(a.sparkline(samples, c)),
			)
		})
	}
}

// sparkline draws the last samples as a filled area with a 2dp line.
func (a *App) sparkline(samples []float64, c color.NRGBA) layout.Widget {
	return func(gtx C) D {
		w, h := gtx.Constraints.Max.X, gtx.Dp(34)
		size := image.Pt(w, h)
		defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, alpha(a.pal.Surface2, 0x60), clip.Rect{Min: image.Pt(0, h-1), Max: size}.Op())
		n := 40
		if len(samples) < 2 {
			return D{Size: size}
		}
		maximum := 0.0
		for _, s := range samples {
			maximum = math.Max(maximum, s)
		}
		if maximum == 0 {
			return D{Size: size}
		}
		step := float32(w) / float32(n-1)
		x0 := float32(n-len(samples)) * step
		pt := func(i int) f32.Point {
			return f32.Pt(x0+float32(i)*step, float32(h-2)-float32(samples[i]/maximum)*float32(h-6))
		}
		// Two clip.Paths must not record at the same time, so build them in turn.
		var area clip.Path
		area.Begin(gtx.Ops)
		area.MoveTo(f32.Pt(x0, float32(h)))
		for i := range samples {
			area.LineTo(pt(i))
		}
		area.LineTo(f32.Pt(float32(w), float32(h)))
		area.Close()
		paint.FillShape(gtx.Ops, alpha(c, 0x30), clip.Outline{Path: area.End()}.Op())
		var line clip.Path
		line.Begin(gtx.Ops)
		line.MoveTo(pt(0))
		for i := 1; i < len(samples); i++ {
			line.LineTo(pt(i))
		}
		paint.FillShape(gtx.Ops, c, clip.Stroke{Path: line.End(), Width: float32(gtx.Dp(2))}.Op())
		return D{Size: size}
	}
}

func (a *App) connectionDetails(p openvpn3.Profile) layout.Widget {
	server := "—"
	if p.ServerIP != "" {
		server = p.ServerIP
		if p.ServerPort != 0 {
			server += fmt.Sprintf(":%d", p.ServerPort)
		}
	}
	duration := "—"
	if p.State == openvpn3.Connected && !p.CreatedAt.IsZero() {
		duration = formatDuration(time.Since(p.CreatedAt))
	}
	item := func(ico, k, v string) layout.Widget {
		if v == "" {
			v = "—"
		}
		return func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(a.row(a.icon(ico, a.pal.Overlay1, unit.Sp(11)), a.small(k, a.pal.Overlay1))),
				layout.Rigid(a.mono(v, a.pal.Subtext1, unit.Sp(13))),
			)
		}
	}
	return func(gtx C) D {
		return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Rigid(item(icoServer, "SERVER", server)),
			layout.Rigid(item(icoGlobe, "PROTOCOL", p.Protocol)),
			layout.Rigid(item(icoLink, "INTERFACE", p.Device)),
			layout.Rigid(item(icoClock, "TIME", duration)),
		)
	}
}

func (a *App) oauthBox(rawURL string) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return a.box(gtx, alpha(a.pal.Peach, 0x18), a.pal.Peach, unit.Dp(8), layout.UniformInset(unit.Dp(12)), func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(a.row(a.icon(icoExternal, a.pal.Peach, unit.Sp(13)), a.mono("BROWSER SIGN-IN", a.pal.Peach, unit.Sp(12)))),
						layout.Rigid(a.small("Complete authentication in the browser window.", a.pal.Text)),
						layout.Rigid(a.small(rawURL, a.pal.Overlay1)),
					)
				}),
				layout.Rigid(a.btn(&a.browserBtn, icoExternal, "Open page", btnSecondary)),
			)
		})
	}
}

func (a *App) stateColor(state openvpn3.State) color.NRGBA {
	switch state {
	case openvpn3.Connected:
		return a.pal.Green
	case openvpn3.Connecting, openvpn3.WaitingForBrowser, openvpn3.Disconnecting:
		return a.pal.Peach
	case openvpn3.Failed:
		return a.pal.Red
	default:
		return a.pal.Overlay1
	}
}

func formatBytes(value float64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%.0f B", value)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	exponent := int(math.Floor(math.Log(value) / math.Log(unit)))
	exponent = min(max(exponent, 1), len(units))
	return fmt.Sprintf("%.1f %s", value/math.Pow(unit, float64(exponent)), units[exponent-1])
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "—"
	}
	total := int(d.Seconds())
	return fmt.Sprintf("%02d:%02d:%02d", total/3600, total%3600/60, total%60)
}
