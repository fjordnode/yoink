package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const statusDuration = time.Second

func execWlCopy(data []byte) *exec.Cmd {
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(string(data))
	return cmd
}

func main() {
	// Capture source window before TUI opens
	source := captureSourceWindow()

	// Load clipboard entries
	entries, err := cliphistList()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Load theme
	loadTheme()

	// Load and reconcile metadata
	meta := loadMetadata()
	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = e.ID
	}
	meta.reconcile(ids)
	_ = meta.save()

	// Create and run TUI
	m := newModel(entries, meta, source)
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
