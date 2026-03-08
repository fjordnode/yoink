package main

import (
	"bytes"
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
	// Capture source window before TUI opens
	source := captureSourceWindow()

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
		pasteToWindow(source, finalModel.pendingPaste)
	}
}
