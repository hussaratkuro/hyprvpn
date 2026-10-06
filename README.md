# hyprvpn

Gio GUI version of the `charmvpn` OpenVPN 3 client, following the HyDE
wallbash palette live. The `charmvpn-tray` process is untouched and keeps
working next to it; both talk to the same OpenVPN 3 D-Bus services.

Layout: profile list on the left (state dot, status), detail on the right
with a power button (connect / disconnect), received / sent cards with
rates and sparklines, server / protocol / interface / time, an OAuth box
with "Open page" when a browser sign-in is pending, and notices.

Everything is clickable; keyboard shortcuts match the TUI:

| Key | Action |
|---|---|
| `↑` / `↓`, `j` / `k` | Select a profile |
| `Enter` or `Space` | Connect or disconnect |
| `o` | Reopen the OAuth page |
| `i` | Import an `.ovpn` profile |
| `Delete` / `Backspace` | Remove an inactive profile |
| `r` | Refresh now |
| `q`, `ctrl+q` | Quit without disconnecting |

During authentication `ctrl+g` (or the "From vault" button) fills the prompt
from a linked hyprvault / gopass entry and `ctrl+r` changes the link. Links
live in `~/.config/hyprvpn/credential-refs.json` (hyprvpn's own file since 2026-09-25; the TUI keeps `~/.config/charmvpn/`). The
vault password goes to `hyprvault credential get` (falls back to `gopass`)
through stdin and stays in memory for 15 minutes.

`HYPRVPN_DEMO=1 hyprvpn` runs against a fake backend with three profiles to
preview the UI without a VPN. `TUI_THEME=catppuccin` forces the Catppuccin
Mocha palette.

Build: `./install-local.sh` (uses `-tags novulkan`).

## Dependencies

### Go modules

- `gioui.org` v0.10 — GUI toolkit
- `github.com/godbus/dbus/v5` — talks to OpenVPN 3 Linux over the system D-Bus

### Build

- Go 1.27 (see `go.mod`), a C compiler and `pkg-config` (Gio uses CGO on Linux)
- Wayland: `wayland`, `wayland-protocols`, `libxkbcommon`
- X11 fallback headers (Gio builds both backends): `libx11`, `libxcursor`, `libxfixes`
- OpenGL ES / EGL from `mesa`; Vulkan is disabled with `-tags novulkan`, so no Vulkan headers are needed

### Runtime

- A Wayland compositor (developed on Hyprland); the `.desktop` file uses `StartupWMClass`
- Fonts: JetBrainsMono Nerd Font Propo and Mono in `~/.local/share/fonts/JetBrainsMono/` (HyDE ships them; the Go fonts are the fallback), Noto Sans Symbols / Symbols2 / Noto Sans Mono as glyph fallback (optional)
- HyDE wallbash palette from `~/.cache/hyde/wallbash/shell-colors` (optional; without it, or with `TUI_THEME=catppuccin`, Catppuccin Mocha is used)
- OpenVPN 3 Linux (`openvpn3` package: the `net.openvpn.v3.*` D-Bus services); without it the profile list says "OpenVPN 3 is not reachable"
- `hyprvault` (or `gopass`) in PATH for vault-linked credentials
- `xdg-open` for opening web-auth pages

## License

MIT, see `LICENSE`.
