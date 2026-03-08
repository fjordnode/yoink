package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"syscall"
)

type SourceWindow struct {
	Address string
	Class   string
}

func captureSourceWindow() SourceWindow {
	out, err := exec.Command("hyprctl", "activewindow", "-j").Output()
	if err != nil {
		return SourceWindow{}
	}

	var data struct {
		Address string `json:"address"`
		Class   string `json:"class"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		return SourceWindow{}
	}

	return SourceWindow{
		Address: data.Address,
		Class:   strings.ToLower(data.Class),
	}
}

func pasteToWindow(sw SourceWindow, rawLine string) {
	// Decode and copy to clipboard before exiting
	data, err := cliphistDecode(rawLine)
	if err != nil {
		return
	}
	cmd := exec.Command("wl-copy")
	cmd.Stdin = bytes.NewReader(data)
	if err := cmd.Run(); err != nil {
		return
	}

	// Only paste-back if we have a valid source window to target
	if sw.Address == "" {
		return
	}

	// Fork detached process that waits for our kitty window to close,
	// then focuses source and pastes
	script := buildPasteScript(sw)
	c := exec.Command("bash", "-c", script)
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	_ = c.Start()
}

func buildPasteScript(sw SourceWindow) string {
	var pasteCmd string
	switch {
	case strings.Contains(sw.Class, "brave"):
		pasteCmd = "wtype -s 20 -M ctrl -k v -m ctrl"
	case strings.Contains(sw.Class, "kitty"),
		strings.Contains(sw.Class, "alacritty"),
		strings.Contains(sw.Class, "ghostty"),
		strings.Contains(sw.Class, "wezterm"),
		strings.Contains(sw.Class, "foot"):
		pasteCmd = "wtype -s 20 -M ctrl -M shift -k v -m shift -m ctrl"
	default:
		pasteCmd = "wtype -s 20 -M shift -k Insert -m shift"
	}

	// Poll for the clipboard TUI window to disappear (up to 500ms),
	// then focus source and paste immediately. No fixed sleep.
	return `
for i in $(seq 1 25); do
  if ! hyprctl clients -j 2>/dev/null | jq -e '.[] | select(.class == "yoink")' >/dev/null 2>&1; then
    break
  fi
  sleep 0.02
done
hyprctl dispatch focuswindow 'address:` + sw.Address + `' >/dev/null 2>&1
sleep 0.02
` + pasteCmd
}
