package ui

import (
	"fmt"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
)

// layoutModal dims the dashboard and centers the active dialog on top.
func (a *App) layoutModal(gtx C) D {
	// The scrim is an input area too, so clicks outside the dialog stop at it.
	paint.FillShape(gtx.Ops, alpha(a.pal.Crust, 0xA0), clip.Rect{Max: gtx.Constraints.Max}.Op())
	scrim := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, &scrimTag)
	scrim.Pop()

	for {
		ev, ok := a.input.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok && !a.busy {
			a.submitModal()
		}
	}
	if a.okBtn.Clicked(gtx) && !a.busy {
		a.submitModal()
	}
	if a.cancelBtn.Clicked(gtx) {
		a.cancelModal()
	}
	if a.yesBtn.Clicked(gtx) && !a.busy {
		a.confirmDelete()
	}
	if a.noBtn.Clicked(gtx) {
		a.mode = modeDashboard
	}
	if a.linkBtn.Clicked(gtx) && a.mode == modeCredentials {
		if !a.tryLinkedCredential() {
			a.openCredentialReference()
		}
	}
	needsInput := a.mode != modeDelete
	if needsInput && !gtx.Focused(&a.input) {
		a.focus(gtx, &a.input)
	}

	var ico, title, body, hint, ok string
	switch a.mode {
	case modeImport:
		ico, title, body, hint, ok = icoFile, "Import VPN profile", "Path to an OpenVPN .ovpn file.", "/home/user/vpn/work.ovpn", "Import"
	case modeCredentials:
		desc := "Authentication input"
		if a.request < len(a.requests) {
			desc = a.requests[a.request].Description
			if strings.TrimSpace(desc) == "" {
				desc = a.requests[a.request].Name
			}
		}
		ico, title, body, ok = icoLock, "VPN authentication", fmt.Sprintf("%s  ·  %d/%d", desc, a.request+1, len(a.requests)), "Continue"
	case modeCredentialRef:
		ico, title, body, hint, ok = icoLink, "Link vault credential", "hyprvault entry ID or unique title. Leave blank to remove the link.", "entry id or title", "Save and unlock"
	case modeVaultPassword:
		ico, title, body, hint, ok = icoUnlock, "Unlock vault", "Vault password, cached in memory for 15 minutes.", "vault password", "Unlock and fill"
	case modeDelete:
		p, _ := a.selectedProfile()
		ico, title, body = icoDelete, "Remove profile", "Remove \""+p.Name+"\" from OpenVPN 3?"
	}

	return layout.Center.Layout(gtx, a.narrow(460, a.panel(func(gtx C) D {
		ws := []layout.Widget{
			a.row(a.iconDisc(ico, a.pal.Mauve, 32), a.heading(title)),
			a.text(body, a.pal.Subtext0),
		}
		if needsInput {
			ws = append(ws, a.editor(&a.input, hint, true))
		}
		switch a.mode {
		case modeDelete:
			ws = append(ws, a.row(a.btn(&a.yesBtn, icoDelete, "Remove", btnDanger), a.btn(&a.noBtn, "", "Keep", btnSecondary)), a.small("y remove · n keep · esc", a.pal.Overlay0))
		case modeCredentials:
			ws = append(ws, a.row(a.btn(&a.okBtn, icoCheck, ok, btnPrimary), a.btn(&a.linkBtn, icoKey, "From vault", btnSecondary), a.btn(&a.cancelBtn, icoTimes, "Cancel", btnSecondary)),
				a.small("enter continue · ctrl+g fill from vault · ctrl+r change link · esc cancels the connection", a.pal.Overlay0))
		default:
			ws = append(ws, a.row(a.btn(&a.okBtn, icoCheck, ok, btnPrimary), a.btn(&a.cancelBtn, icoTimes, "Cancel", btnSecondary)), a.small("enter confirm · esc back", a.pal.Overlay0))
		}
		if a.errorText != "" {
			ws = append(ws, a.status(a.errorText, ""))
		}
		if a.busy {
			ws = append(ws, a.small("working…", a.pal.Peach))
		}
		return a.column(gtx, ws...)
	})))
}

// scrimTag is the input area that swallows clicks outside the dialog.
var scrimTag int

func (a *App) focus(gtx C, tag any) { gtx.Execute(key.FocusCmd{Tag: tag}) }
