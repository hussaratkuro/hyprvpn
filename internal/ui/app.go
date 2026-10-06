// Package ui is the Gio front end of hyprvpn: an OpenVPN Connect style
// dashboard that follows the HyDE wallbash palette live.
package ui

import (
	"strings"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/godbus/dbus/v5"

	"github.com/hussaratkuro/hyprvpn/internal/desktop"
	"github.com/hussaratkuro/hyprvpn/internal/openvpn3"
	"github.com/hussaratkuro/hyprvpn/internal/theme"
)

const refreshInterval = time.Second

type backend interface {
	Profiles() ([]openvpn3.Profile, error)
	Import(string) error
	Remove(dbus.ObjectPath) error
	BeginConnect(dbus.ObjectPath) (openvpn3.ConnectResult, error)
	ContinueConnect(dbus.ObjectPath) (openvpn3.ConnectResult, error)
	Provide(dbus.ObjectPath, openvpn3.InputRequest, string) error
	PendingInputs(dbus.ObjectPath) ([]openvpn3.InputRequest, error)
	Disconnect(dbus.ObjectPath) error
}

type mode int

const (
	modeDashboard mode = iota
	modeImport
	modeCredentials
	modeCredentialRef
	modeVaultPassword
	modeDelete
)

type App struct {
	w       *app.Window
	th      *material.Theme
	pal     theme.Palette
	client  backend
	version string
	posted  chan func()
	pending atomic.Int32

	profiles []openvpn3.Profile
	selected int
	loading  bool
	busy     bool
	mode     mode

	input       widget.Editor
	requests    []openvpn3.InputRequest
	request     int
	authSession dbus.ObjectPath

	notice      string
	errorText   string
	loadError   bool
	lastAuthURL string
	history     map[dbus.ObjectPath]*trafficHistory

	credentialRefs   map[string]string
	credentialRef    string
	credentialMaster string
	credentialUntil  time.Time

	// widgets
	rowClicks                                              []widget.Clickable
	list                                                   widget.List
	importBtn, refreshBtn, powerBtn, browserBtn, removeBtn widget.Clickable
	okBtn, cancelBtn, linkBtn, yesBtn, noBtn               widget.Clickable
}

type trafficHistory struct {
	lastIn, lastOut uint64
	lastAt          time.Time
	rateIn, rateOut float64
	inSamples       []float64
	outSamples      []float64
}

func Run(client backend, version string) error {
	w := new(app.Window)
	w.Option(app.Title("hyprvpn"), app.Size(unit.Dp(900), unit.Dp(600)), app.MinSize(unit.Dp(560), unit.Dp(420)))
	refs, _ := loadCredentialRefs()
	a := &App{
		w: w, th: material.NewTheme(), client: client, version: version, posted: make(chan func(), 32),
		loading: true, history: make(map[dbus.ObjectPath]*trafficHistory), credentialRefs: refs,
		input: widget.Editor{SingleLine: true, Submit: true},
	}
	a.list.Axis = layout.Vertical
	a.th.Shaper = text.NewShaper(text.WithCollection(loadFonts()))
	a.th.Face = nerdFont
	a.applyTheme(theme.Current())
	theme.Watch(func(p theme.Palette) { a.post(func() { a.applyTheme(p) }) })
	a.loadProfiles()
	go func() {
		for range time.Tick(refreshInterval) {
			a.post(a.tick)
		}
	}()

	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			a.drain()
			gtx := app.NewContext(&ops, e)
			a.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

// post runs f on the UI goroutine before the next frame. Gio coalesces
// Invalidate calls until the app has gone idle again, so an Invalidate that
// lands while a frame is being processed is dropped; the retries below make
// sure a posted result is never left waiting for the next input event.
func (a *App) post(f func()) {
	a.posted <- f
	a.pending.Add(1)
	a.w.Invalidate()
	go func() {
		for i := 0; i < 12 && a.pending.Load() > 0; i++ {
			time.Sleep(25 * time.Millisecond)
			a.w.Invalidate()
		}
	}()
}

func (a *App) drain() {
	for {
		select {
		case f := <-a.posted:
			f()
			a.pending.Add(-1)
		default:
			return
		}
	}
}

func (a *App) applyTheme(p theme.Palette) {
	a.pal = p
	a.th.Palette = material.Palette{Bg: p.Base, Fg: p.Text, ContrastBg: p.Mauve, ContrastFg: p.OnAccent}
}

func (a *App) layout(gtx C) D {
	paint.FillShape(gtx.Ops, a.pal.Base, clip.Rect{Max: gtx.Constraints.Max}.Op())
	// Key handler area goes first so it sits beneath every clickable widget.
	keyArea := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, a)
	keyArea.Pop()
	a.layoutDashboard(gtx)
	if a.mode != modeDashboard {
		a.layoutModal(gtx)
	}
	a.globalKeys(gtx)
	return D{Size: gtx.Constraints.Max}
}

// ---- periodic refresh and background work ----

func (a *App) tick() {
	if a.busy || a.loading || a.mode == modeCredentials || a.mode == modeCredentialRef || a.mode == modeVaultPassword {
		return
	}
	a.loading = true
	a.loadProfiles()
}

func (a *App) loadProfiles() {
	go func() {
		profiles, err := a.client.Profiles()
		at := time.Now()
		a.post(func() { a.profilesLoaded(profiles, err, at) })
	}()
}

func (a *App) profilesLoaded(profiles []openvpn3.Profile, err error, at time.Time) {
	a.loading = false
	if err != nil {
		a.errorText, a.loadError = err.Error(), true
		return
	}
	if a.loadError {
		a.errorText, a.loadError = "", false
	}
	selectedPath := a.selectedProfilePath()
	a.profiles = profiles
	a.selected = max(0, indexOfPath(a.profiles, selectedPath))
	if len(a.rowClicks) < len(a.profiles) {
		a.rowClicks = make([]widget.Clickable, len(a.profiles))
	}
	a.updateTraffic(at)
	profile, ok := a.selectedProfile()
	if !ok {
		return
	}
	if profile.AuthURL != "" && profile.AuthURL != a.lastAuthURL {
		a.lastAuthURL = profile.AuthURL
		a.notice = "Authentication page opened in browser"
		a.openBrowser(profile.AuthURL)
		return
	}
	if a.mode == modeDashboard && !a.busy && profile.SessionPath.IsValid() && profile.Status == "Authentication required" {
		a.busy = true
		session := profile.SessionPath
		go func() {
			requests, err := a.client.PendingInputs(session)
			a.post(func() {
				a.busy = false
				if err != nil {
					a.fail(err)
					return
				}
				if len(requests) > 0 {
					a.authSession = session
					a.beginCredentials(requests)
					a.tryLinkedCredential()
				}
			})
		}()
	}
}

func (a *App) fail(err error) {
	a.errorText, a.loadError = err.Error(), false
}

func (a *App) connectResult(result openvpn3.ConnectResult, err error) {
	a.busy = false
	if err != nil {
		a.fail(err)
		a.loadProfiles()
		return
	}
	a.authSession = result.SessionPath
	if len(result.Requests) > 0 {
		a.beginCredentials(result.Requests)
		a.tryLinkedCredential()
		return
	}
	a.notice = "Connecting…"
	a.loadProfiles()
}

func (a *App) beginConnect(path dbus.ObjectPath) {
	a.errorText, a.notice, a.busy = "", "", true
	go func() {
		result, err := a.client.BeginConnect(path)
		a.post(func() { a.connectResult(result, err) })
	}()
}

func (a *App) continueConnect(path dbus.ObjectPath) {
	a.busy = true
	go func() {
		result, err := a.client.ContinueConnect(path)
		a.post(func() { a.connectResult(result, err) })
	}()
}

func (a *App) disconnect(path dbus.ObjectPath) {
	a.errorText, a.notice, a.busy = "", "", true
	a.action("disconnect", func() error { return a.client.Disconnect(path) })
}

func (a *App) action(kind string, run func() error) {
	a.busy = true
	go func() {
		err := run()
		a.post(func() {
			a.busy = false
			if err != nil {
				a.fail(err)
			} else {
				a.notice = map[string]string{"import": "Profile imported", "remove": "Profile removed", "disconnect": "VPN disconnected"}[kind]
			}
			a.mode = modeDashboard
			a.loadProfiles()
		})
	}()
}

func (a *App) provideInput(value string) {
	if a.request >= len(a.requests) {
		return
	}
	req := a.requests[a.request]
	a.busy = true
	go func() {
		err := a.client.Provide(a.authSession, req, value)
		a.post(func() {
			a.busy = false
			if err != nil {
				a.fail(err)
				return
			}
			a.request++
			if a.request < len(a.requests) {
				a.configureCredentialInput()
				a.tryLinkedCredential()
				return
			}
			a.mode = modeDashboard
			a.continueConnect(a.authSession)
		})
	}()
}

func (a *App) openBrowser(rawURL string) {
	go func() {
		err := desktop.OpenBrowser(rawURL)
		a.post(func() {
			if err != nil {
				a.errorText = "Could not open browser: " + err.Error()
				a.notice = "Use the button to try again; the URL stays on screen"
			} else {
				a.notice = "Complete sign-in in your browser"
			}
		})
	}()
}

// ---- credentials ----

func (a *App) beginCredentials(requests []openvpn3.InputRequest) {
	a.requests, a.request, a.mode = requests, 0, modeCredentials
	a.configureCredentialInput()
}

func (a *App) configureCredentialInput() {
	a.input.SetText("")
	a.input.Mask = 0
	if a.request < len(a.requests) && a.requests[a.request].Hidden {
		a.input.Mask = '•'
	}
}

func (a *App) credentialProfileName() string {
	profile, ok := a.selectedProfile()
	if !ok {
		return ""
	}
	return profile.Name
}

// tryLinkedCredential fills the current prompt from the linked vault entry,
// if this profile has one.
func (a *App) tryLinkedCredential() bool {
	ref := strings.TrimSpace(a.credentialRefs[a.credentialProfileName()])
	if ref == "" {
		return false
	}
	a.credentialRef = ref
	a.startVaultCredential()
	return true
}

func (a *App) startVaultCredential() {
	if a.credentialMaster != "" && time.Now().Before(a.credentialUntil) {
		a.resolveCredential(a.credentialMaster)
		return
	}
	a.mode = modeVaultPassword
	a.input.SetText("")
	a.input.Mask = '•'
}

func (a *App) resolveCredential(master string) {
	a.busy = true
	ref := a.credentialRef
	go func() {
		cred, err := resolveVPNCredential(ref, master)
		a.post(func() {
			a.busy = false
			a.mode = modeCredentials
			if err != nil {
				a.credentialMaster, a.credentialUntil = "", time.Time{}
				a.errorText = "vault: " + err.Error()
				a.configureCredentialInput()
				return
			}
			a.credentialMaster, a.credentialUntil = master, time.Now().Add(15*time.Minute)
			value := cred.Password
			if a.request < len(a.requests) && credentialWantsUsername(a.requests[a.request]) {
				if cred.Username == "" {
					a.errorText = "vault entry has no username"
					a.configureCredentialInput()
					return
				}
				value = cred.Username
			}
			a.configureCredentialInput()
			a.input.SetText(value)
			a.notice = "Filled authentication input from the encrypted vault"
			a.errorText = ""
		})
	}()
}

func (a *App) openCredentialReference() {
	a.credentialRef = a.credentialRefs[a.credentialProfileName()]
	a.mode = modeCredentialRef
	a.input.Mask = 0
	a.input.SetText(a.credentialRef)
}

func (a *App) saveCredentialReference() {
	ref := strings.TrimSpace(a.input.Text())
	name := a.credentialProfileName()
	if ref == "" {
		delete(a.credentialRefs, name)
		if err := saveCredentialRefs(a.credentialRefs); err != nil {
			a.errorText = "Save vault link: " + err.Error()
		} else {
			a.notice = "Removed vault link from VPN profile"
		}
		a.mode = modeCredentials
		a.configureCredentialInput()
		return
	}
	a.credentialRefs[name] = ref
	if err := saveCredentialRefs(a.credentialRefs); err != nil {
		a.errorText = "Save vault link: " + err.Error()
		return
	}
	a.credentialRef = ref
	a.startVaultCredential()
}

// ---- modal actions ----

func (a *App) submitModal() {
	switch a.mode {
	case modeImport:
		path := strings.TrimSpace(a.input.Text())
		if path == "" {
			a.errorText = "Enter the path to an .ovpn file"
			return
		}
		a.action("import", func() error { return a.client.Import(path) })
	case modeCredentials:
		a.provideInput(a.input.Text())
	case modeCredentialRef:
		a.saveCredentialReference()
	case modeVaultPassword:
		master := a.input.Text()
		if master == "" {
			a.errorText = "Vault password is required"
			return
		}
		a.resolveCredential(master)
	case modeDelete:
		a.confirmDelete()
	}
}

func (a *App) cancelModal() {
	switch a.mode {
	case modeCredentials:
		a.mode = modeDashboard
		a.disconnect(a.authSession)
	case modeCredentialRef, modeVaultPassword:
		a.mode = modeCredentials
		a.configureCredentialInput()
	default:
		a.mode = modeDashboard
	}
}

func (a *App) confirmDelete() {
	profile, ok := a.selectedProfile()
	if !ok {
		a.mode = modeDashboard
		return
	}
	a.action("remove", func() error { return a.client.Remove(profile.Path) })
}

func (a *App) openImport() {
	a.mode, a.errorText = modeImport, ""
	a.input.Mask = 0
	a.input.SetText("")
}

func (a *App) togglePower() {
	profile, ok := a.selectedProfile()
	if !ok || a.busy {
		return
	}
	if profile.SessionPath.IsValid() {
		a.disconnect(profile.SessionPath)
	} else {
		a.beginConnect(profile.Path)
	}
}

func (a *App) refresh() {
	a.loading = true
	a.loadProfiles()
}

// ---- keys ----

func (a *App) globalKeys(gtx C) {
	filters := []event.Filter{key.Filter{Name: key.NameEscape}, key.Filter{Name: "Q", Required: key.ModCtrl}}
	if a.mode == modeDashboard {
		for _, n := range []key.Name{key.NameUpArrow, key.NameDownArrow, key.NameReturn, key.NameSpace, key.NameDeleteBackward, key.NameDeleteForward, "J", "K", "I", "R", "O", "Q"} {
			filters = append(filters, key.Filter{Name: n})
		}
	} else if a.mode == modeDelete {
		filters = append(filters, key.Filter{Name: "Y"}, key.Filter{Name: "N"})
	} else if a.mode == modeCredentials {
		filters = append(filters, key.Filter{Name: "G", Required: key.ModCtrl}, key.Filter{Name: "R", Required: key.ModCtrl})
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		k, ok := ev.(key.Event)
		if !ok || k.State != key.Press {
			continue
		}
		switch {
		case k.Name == key.NameEscape:
			if a.mode != modeDashboard {
				a.cancelModal()
			}
		case k.Name == "Q" && (k.Modifiers.Contain(key.ModCtrl) || a.mode == modeDashboard):
			a.w.Perform(system.ActionClose)
		case a.mode == modeDelete:
			if k.Name == "Y" {
				a.confirmDelete()
			} else {
				a.mode = modeDashboard
			}
		case a.mode == modeCredentials && k.Modifiers.Contain(key.ModCtrl):
			if k.Name == "G" && a.tryLinkedCredential() {
				continue
			}
			a.openCredentialReference()
		case a.mode == modeDashboard && !a.busy:
			a.dashboardKey(k)
		}
	}
}

func (a *App) dashboardKey(k key.Event) {
	switch k.Name {
	case key.NameUpArrow, "K":
		if a.selected > 0 {
			a.selected--
		}
		a.notice, a.errorText = "", ""
	case key.NameDownArrow, "J":
		if a.selected+1 < len(a.profiles) {
			a.selected++
		}
		a.notice, a.errorText = "", ""
	case "I":
		a.openImport()
	case "R":
		a.refresh()
	case "O":
		if p, ok := a.selectedProfile(); ok && p.AuthURL != "" {
			a.openBrowser(p.AuthURL)
		}
	case key.NameDeleteBackward, key.NameDeleteForward:
		if p, ok := a.selectedProfile(); ok && p.SessionPath == "" {
			a.mode = modeDelete
		}
	case key.NameReturn, key.NameSpace:
		a.togglePower()
	}
}

// ---- state helpers ----

func (a *App) updateTraffic(at time.Time) {
	for _, profile := range a.profiles {
		h := a.history[profile.Path]
		if h == nil {
			h = &trafficHistory{}
			a.history[profile.Path] = h
		}
		if !h.lastAt.IsZero() {
			seconds := at.Sub(h.lastAt).Seconds()
			if seconds > 0 && profile.BytesIn >= h.lastIn && profile.BytesOut >= h.lastOut {
				h.rateIn = float64(profile.BytesIn-h.lastIn) / seconds
				h.rateOut = float64(profile.BytesOut-h.lastOut) / seconds
			}
		}
		if profile.State != openvpn3.Connected {
			h.rateIn, h.rateOut = 0, 0
		}
		h.lastIn, h.lastOut, h.lastAt = profile.BytesIn, profile.BytesOut, at
		h.inSamples = appendSample(h.inSamples, h.rateIn, 40)
		h.outSamples = appendSample(h.outSamples, h.rateOut, 40)
	}
}

func (a *App) selectedProfile() (openvpn3.Profile, bool) {
	if a.selected < 0 || a.selected >= len(a.profiles) {
		return openvpn3.Profile{}, false
	}
	return a.profiles[a.selected], true
}

func (a *App) selectedProfilePath() dbus.ObjectPath {
	p, ok := a.selectedProfile()
	if !ok {
		return ""
	}
	return p.Path
}

func indexOfPath(profiles []openvpn3.Profile, path dbus.ObjectPath) int {
	if path == "" && len(profiles) > 0 {
		return 0
	}
	for i := range profiles {
		if profiles[i].Path == path {
			return i
		}
	}
	return -1
}

func appendSample(values []float64, value float64, maximum int) []float64 {
	values = append(values, value)
	if len(values) > maximum {
		values = values[len(values)-maximum:]
	}
	return values
}
