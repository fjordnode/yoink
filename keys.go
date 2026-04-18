package main

import tea "github.com/charmbracelet/bubbletea"

type keyAction int

const (
	keyNone keyAction = iota
	keyPaste
	keyCopy
	keyDelete
	keySave
	keyQuit
	keyFullPreview
	keyRefresh
)

func classifyKey(msg tea.KeyMsg) keyAction {
	switch msg.Type {
	case tea.KeyEnter:
		return keyPaste
	case tea.KeyEsc:
		return keyQuit
	case tea.KeyCtrlD:
		return keyDelete
	case tea.KeyCtrlP:
		return keySave
	case tea.KeyCtrlN:
		return keyCopy
	case tea.KeyCtrlY:
		return keyCopy
	case tea.KeyTab:
		return keyFullPreview
	case tea.KeyCtrlR:
		return keyRefresh
	}
	return keyNone
}
