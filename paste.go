package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

type SourceWindow struct {
	Address string
	Class   string
}

func detectCompositor() string {
	if os.Getenv("XDG_CURRENT_DESKTOP") == "niri" {
		return "niri"
	}
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "" {
		return "hyprland"
	}
	return ""
}

func captureSourceWindow() SourceWindow {
	switch detectCompositor() {
	case "niri":
		return captureSourceWindowNiri()
	case "hyprland":
		return captureSourceWindowHyprland()
	}
	return SourceWindow{}
}

func captureSourceWindowHyprland() SourceWindow {
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

func captureSourceWindowNiri() SourceWindow {
	out, err := exec.Command("niri", "msg", "-j", "focused-window").Output()
	if err != nil {
		return SourceWindow{}
	}

	var data struct {
		ID    int    `json:"id"`
		AppID string `json:"app_id"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		return SourceWindow{}
	}

	return SourceWindow{
		Address: strconv.Itoa(data.ID),
		Class:   strings.ToLower(data.AppID),
	}
}

func pasteToWindowDirect(sw SourceWindow) {
	if sw.Address == "" {
		return
	}
	script := buildPasteScript(sw)
	c := exec.Command("bash", "-c", script)
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	_ = c.Start()
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
	case strings.Contains(sw.Class, "brave"),
		strings.Contains(sw.Class, "firefox"),
		strings.Contains(sw.Class, "chromium"):
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

	switch detectCompositor() {
	case "niri":
		return buildNiriPasteScript(sw, pasteCmd)
	default:
		return buildHyprlandPasteScript(sw, pasteCmd)
	}
}

func buildNiriPasteScript(sw SourceWindow, pasteCmd string) string {
	return `
for i in $(seq 1 25); do
  if ! niri msg -j windows 2>/dev/null | jq -e '.[] | select(.app_id == "yoink")' >/dev/null 2>&1; then
    break
  fi
  sleep 0.02
done
niri msg action focus-window --id ` + sw.Address + ` >/dev/null 2>&1
sleep 0.02
` + pasteCmd
}

func buildHyprlandPasteScript(sw SourceWindow, pasteCmd string) string {
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
