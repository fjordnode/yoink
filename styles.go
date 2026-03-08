package main

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/lipgloss"
)

type ThemeColors struct {
	Accent              string `toml:"accent"`
	Foreground          string `toml:"foreground"`
	Background          string `toml:"background"`
	SelectionForeground string `toml:"selection_foreground"`
	SelectionBackground string `toml:"selection_background"`
	Color0              string `toml:"color0"`
	Color8              string `toml:"color8"`
	Color15             string `toml:"color15"`
}

var theme ThemeColors

func loadTheme() {
	theme = ThemeColors{
		Accent:              "#82FB9C",
		Foreground:          "#ddf7ff",
		Background:          "#0B0C16",
		SelectionForeground: "#0B0C16",
		SelectionBackground: "#ddf7ff",
		Color0:              "#0B0C16",
		Color8:              "#6a6e95",
		Color15:             "#ddf7ff",
	}

	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".config", "omarchy", "current", "theme", "colors.toml")
	_, _ = toml.DecodeFile(p, &theme)
}

// Style helpers
func accentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent))
}

func dimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Color8))
}

func fgStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Foreground))
}

func selectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.SelectionForeground)).
		Background(lipgloss.Color(theme.SelectionBackground))
}

func dimSelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Color8)).
		Background(lipgloss.Color(theme.SelectionBackground))
}

func searchStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Foreground)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Color8)).
		Padding(0, 1)
}

func previewBorderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(theme.Color8)).
		PaddingLeft(1)
}

func helpStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Color8))
}

func helpKeyStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent))
}

func statusStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent)).
		Bold(true)
}
