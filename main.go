package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const statusDuration = time.Second

func execWlCopy(data []byte) *exec.Cmd {
	cmd := exec.Command("wl-copy")
	cmd.Stdin = bytes.NewReader(data)
	return cmd
}

func main() {
	sourceID := flag.String("source-id", "", "Source window ID (niri window id or hyprland address)")
	sourceClass := flag.String("source-class", "", "Source window app-id/class")
	flag.Parse()

	// Use passed-in source window, or auto-detect
	var source SourceWindow
	if *sourceID != "" {
		source = SourceWindow{Address: *sourceID, Class: *sourceClass}
	} else {
		source = captureSourceWindow()
	}

	// Load theme
	loadTheme()

	// Load metadata (fast, local JSON)
	meta := loadMetadata()

	// Launch TUI immediately with empty entries — loads async
	m := newModel(nil, meta, source)
	p := tea.NewProgram(m, tea.WithAltScreen())

	result, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Clear any kitty images on exit
	clearKittyImages()

	// Handle paste action
	if finalModel, ok := result.(model); ok && finalModel.pendingPaste != "" {
		if finalModel.pendingPaste == "__snippet__" {
			pasteToWindowDirect(source)
		} else {
			pasteToWindow(source, finalModel.pendingPaste)
		}
	}
}
