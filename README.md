# yoink

A clipboard manager TUI for Wayland. Wraps [cliphist](https://github.com/sentriz/cliphist) with a two-pane interface — list on the left, preview on the right — with image previews via the Kitty graphics protocol. Works with Hyprland and Niri.

<p>
  <img src="screenshots/yoink-text-preview.png" width="32%">
  <img src="screenshots/yoink-image-preview.png" width="32%">
  <img src="screenshots/yoink-fullscreen.png" width="32%">
</p>

## Features

- Browse and search clipboard history with fuzzy filtering
- Live text preview with word-wrap
- Image preview via Kitty graphics protocol (works in Kitty, Ghostty, WezTerm)
- Full-screen image preview with Tab (cycle images with arrows)
- Tab on text entries opens in neovim for full scroll/search/yank
- Persistent saved snippets that survive cliphist rotation
- Annotate images with [satty](https://github.com/gabm/Satty) (Ctrl+E)
- Paste-back to the source window on Enter
- Niri and Hyprland compositor support
- Themeable via TOML color scheme

## Requirements

- [cliphist](https://github.com/sentriz/cliphist) — clipboard history backend
- [wl-copy](https://github.com/bugaevc/wl-clipboard) — Wayland clipboard
- [wtype](https://github.com/atx/wtype) — keyboard simulation for paste-back
- [jq](https://jqlang.github.io/jq/) — JSON processing (used by paste-back and watcher)
- [neovim](https://neovim.io/) — text preview/editing (Tab on text entries)
- A terminal with [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/) support for image previews

Optional:
- [satty](https://github.com/gabm/Satty) — screenshot annotation (Ctrl+E on images)

## Install

```bash
go install github.com/fjordnode/yoink@latest
```

Or clone and build manually:

```bash
git clone https://github.com/fjordnode/yoink.git
cd yoink
make install  # builds and copies to ~/.local/bin/
```

## Keybindings

| Key | Action |
|-----|--------|
| `↑` `↓` | Navigate |
| `Enter` | Paste to source window |
| `Ctrl+N` / `Ctrl+Y` | Copy to clipboard |
| `Ctrl+P` | Save as persistent snippet |
| `Ctrl+D` | Delete entry |
| `Ctrl+E` | Annotate image in satty |
| `Ctrl+R` | Refresh list |
| `Tab` | Full-screen preview (images) / open in neovim (text) |
| `Esc` | Quit (or clear search) |
| Type anything | Search/filter |

### Image preview mode (Tab)

Opens the selected image full-screen. Use `↑` `↓` to cycle through images, `Enter` to paste, `Ctrl+N` to copy, any other key to go back.

### Saved snippets

`Ctrl+P` saves the selected entry to `~/.local/share/yoink/snippets/`. Saved entries persist independently of cliphist's history limit and appear at the top of the list with a `★` indicator. Delete saved entries with `Ctrl+D`.

## Source app tracking (optional)

The TUI can display which app each entry was copied from. Install the watcher script:

```bash
curl -sL https://raw.githubusercontent.com/fjordnode/yoink/main/scripts/yoink-watcher -o ~/.local/bin/yoink-watcher && chmod +x ~/.local/bin/yoink-watcher
```

Then add to your compositor autostart:

```bash
# Hyprland
exec-once = wl-paste --watch yoink-watcher

# Niri (config.kdl)
spawn-at-startup "wl-paste" "--watch" "yoink-watcher"
```

Requires `jq`. Supports both Hyprland (`hyprctl`) and Niri (`niri msg`) for source window detection.

## Theme

Create `~/.config/yoink/theme.toml`:

```toml
accent = "#82FB9C"
foreground = "#ddf7ff"
background = "#0B0C16"
selection_foreground = "#0B0C16"
selection_background = "#ddf7ff"
color0 = "#0B0C16"
color8 = "#6a6e95"
color15 = "#ddf7ff"
```

## Launching

Set up as a floating window in your compositor:

```bash
# Hyprland (bindings.conf)
bindd = SUPER SHIFT, V, Clipboard manager, exec, kitty --app-id yoink -e yoink
windowrule = float on, match:class yoink
windowrule = center on, match:class yoink
windowrule = size 1100 750, match:class yoink

# Niri (config.kdl)
window-rule {
    match app-id="yoink"
    open-floating true
    default-column-width { fixed 1100; }
    default-window-height { fixed 750; }
}
```

Also works with Ghostty (`ghostty --class=yoink -e yoink`) or WezTerm (`wezterm start --class yoink -- yoink`).

> **Note:** Image previews require the Kitty graphics protocol. Kitty and Ghostty have full support. WezTerm's implementation is incomplete — text, search, snippets, and paste all work, but image previews do not render.

## License

MIT
