package main

import tea "github.com/charmbracelet/bubbletea"

type keyAction int

const (
	keyNone keyAction = iota
	keyPaste
	keyCopy
	keyDelete
	keyPin
	keyQuit
	keySearch
	keySearchAccept
	keySearchClear
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
		return keyPin
	case tea.KeyCtrlY:
		return keyCopy
	default:
		if msg.String() == "/" {
			return keySearch
		}
	}
	return keyNone
}

func helpBar() string {
	return helpKeyStyle().Render("^D") + helpStyle().Render("=delete  ") +
		helpKeyStyle().Render("^P") + helpStyle().Render("=pin")
}
