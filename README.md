# yoink

A clipboard manager TUI for Wayland/Hyprland. Wraps [cliphist](https://github.com/sentriz/cliphist) with a two-pane interface — list on the left, preview on the right — with image previews via the Kitty graphics protocol.

<p>
  <img src="screenshots/yoink-text-preview.png" width="32%">
  <img src="screenshots/yoink-image-preview.png" width="32%">
  <img src="screenshots/yoink-fullscreen.png" width="32%">
</p>

## Features

- Browse and search clipboard history
- Live text preview with word-wrap
- Image preview via Kitty graphics protocol
- Full-screen image preview with Tab
- Pin entries to keep them at the top
- Paste-back to the source window on Enter
- Themeable via TOML color scheme

## Requirements

- [cliphist](https://github.com/sentriz/cliphist) — clipboard history backend
- [wl-copy](https://github.com/bugaevc/wl-clipboard) — Wayland clipboard
- [hyprctl](https://wiki.hyprland.org/) — window management
- [wtype](https://github.com/atx/wtype) — keyboard simulation for paste-back
- [jq](https://jqlang.github.io/jq/) — JSON processing (used by paste-back and watcher)
- A terminal with [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/) support (Kitty, Ghostty, etc.) for image previews

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
| `Ctrl+Y` | Copy to clipboard |
| `Ctrl+D` | Delete entry |
| `Ctrl+P` | Pin/unpin entry |
| `Tab` | Full-screen image preview |
| `Esc` | Quit (or clear search) |
| Type anything | Search/filter |

**Image preview mode** (`Tab`): Opens the selected image full-screen. Use `↑` `↓` to cycle through images, `Enter` to paste, any other key to go back.

## Source app tracking (optional)

The TUI can display which app each entry was copied from. Install the watcher script and add it to your Hyprland autostart:

```bash
curl -sL https://raw.githubusercontent.com/fjordnode/yoink/main/scripts/yoink-watcher -o ~/.local/bin/yoink-watcher && chmod +x ~/.local/bin/yoink-watcher
```

Then add to your Hyprland autostart:

```
exec-once = wl-paste --watch yoink-watcher
```

Requires `jq` and `hyprctl`. The script runs on every clipboard change, captures the active window class, and writes it to `~/.local/share/yoink/metadata.json`.

## License

MIT
