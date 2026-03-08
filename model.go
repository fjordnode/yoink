package main

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

type model struct {
	entries      []ClipEntry
	filtered     []ClipEntry
	meta         *MetadataStore
	source       SourceWindow
	cursor       int
	prevCursor   int
	searchInput  string
	width        int
	height       int
	pendingPaste string
	status       string
	imageShown   bool
	loaded       bool // whether initial load has completed
}

// Messages
type statusClearMsg struct{}
type imageShownMsg struct{}
type entriesLoadedMsg struct{ entries []ClipEntry }

func newModel(entries []ClipEntry, meta *MetadataStore, source SourceWindow) model {
	m := model{
		entries:    entries,
		meta:       meta,
		source:     source,
		prevCursor: -1,
		loaded:     entries != nil,
	}
	if entries != nil {
		m.reconcileMeta()
		m.applyFilter()
	}
	return m
}

func (m *model) reconcileMeta() {
	ids := make([]string, len(m.entries))
	for i, e := range m.entries {
		ids[i] = e.ID
	}
	m.meta.reconcile(ids)
	_ = m.meta.save()
}

func (m *model) applyFilter() {
	if m.searchInput == "" {
		m.filtered = make([]ClipEntry, len(m.entries))
		copy(m.filtered, m.entries)
	} else {
		previews := make([]string, len(m.entries))
		for i, e := range m.entries {
			previews[i] = e.Preview
		}
		matches := fuzzy.Find(m.searchInput, previews)
		m.filtered = make([]ClipEntry, 0, len(matches))
		for _, match := range matches {
			m.filtered = append(m.filtered, m.entries[match.Index])
		}
	}

	// Pinned first, preserve cliphist order within groups
	pinned := make([]ClipEntry, 0)
	unpinned := make([]ClipEntry, 0)
	for _, e := range m.filtered {
		if m.meta.isPinned(e.ID) {
			pinned = append(pinned, e)
		} else {
			unpinned = append(unpinned, e)
		}
	}
	m.filtered = append(pinned, unpinned...)

	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m model) selectedEntry() *ClipEntry {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return &m.filtered[m.cursor]
}

// loadEntriesCmd runs cliphist list in the background.
func loadEntriesCmd() tea.Msg {
	entries, _ := cliphistList()
	return entriesLoadedMsg{entries: entries}
}

func (m model) Init() tea.Cmd {
	if m.loaded {
		return nil
	}
	return loadEntriesCmd
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.prevCursor = -1
		return m, nil

	case statusClearMsg:
		m.status = ""
		return m, nil

	case imageShownMsg:
		return m, nil

	case entriesLoadedMsg:
		// Merge new entries without disrupting user's position
		selectedID := ""
		if e := m.selectedEntry(); e != nil {
			selectedID = e.ID
		}

		m.entries = msg.entries
		if m.entries == nil {
			m.entries = []ClipEntry{}
		}
		m.loaded = true
		m.reconcileMeta()
		m.applyFilter()

		// Restore cursor to same entry if still present
		if selectedID != "" {
			for i, e := range m.filtered {
				if e.ID == selectedID {
					m.cursor = i
					break
				}
			}
		}
		m.prevCursor = -1
		return m, m.imageCmd()

	case tea.KeyMsg:
		// Search input: backspace edits filter, printable runes append.
		// Everything else falls through to actions/nav.
		if msg.Type == tea.KeyBackspace {
			if len(m.searchInput) > 0 {
				m.searchInput = m.searchInput[:len(m.searchInput)-1]
				m.applyFilter()
				m.prevCursor = -1
				return m, m.imageCmd()
			}
			return m, nil
		}
		if msg.Type == tea.KeyRunes {
			if msg.String() != "/" {
				m.searchInput += string(msg.Runes)
				m.applyFilter()
				m.prevCursor = -1
				return m, m.imageCmd()
			}
		}

		action := classifyKey(msg)
		switch action {
		case keyQuit:
			if m.searchInput != "" {
				m.searchInput = ""
				m.applyFilter()
				m.prevCursor = -1
				return m, m.imageCmd()
			}
			clearKittyImages()
			return m, tea.Quit

		case keySearch:
			m.searchInput = ""
			m.applyFilter()
			m.prevCursor = -1
			return m, m.imageCmd()

		case keyPaste:
			if e := m.selectedEntry(); e != nil {
				clearKittyImages()
				m.pendingPaste = e.RawLine
				return m, tea.Quit
			}

		case keyCopy:
			if e := m.selectedEntry(); e != nil {
				data, err := cliphistDecode(e.RawLine)
				if err == nil {
					cmd := execWlCopy(data)
					if cmd.Run() == nil {
						m.status = "Copied!"
						return m, tea.Tick(statusDuration, func(_ time.Time) tea.Msg {
							return statusClearMsg{}
						})
					}
				}
			}

		case keyDelete:
			if e := m.selectedEntry(); e != nil {
				if err := cliphistDelete(e.RawLine); err == nil {
					m.meta.remove(e.ID)
					_ = m.meta.save()
					for i, entry := range m.entries {
						if entry.ID == e.ID {
							m.entries = append(m.entries[:i], m.entries[i+1:]...)
							break
						}
					}
					m.applyFilter()
					m.prevCursor = -1
					m.status = "Deleted"
					return m, tea.Batch(
						tea.Tick(statusDuration, func(_ time.Time) tea.Msg {
							return statusClearMsg{}
						}),
						m.imageCmd(),
					)
				}
			}

		case keyPin:
			if e := m.selectedEntry(); e != nil {
				m.meta.togglePin(e.ID)
				_ = m.meta.save()
				m.applyFilter()
			}

		default:
			prevCursor := m.cursor
			switch msg.Type {
			case tea.KeyUp:
				if m.cursor > 0 {
					m.cursor--
				}
			case tea.KeyDown:
				if m.cursor < len(m.filtered)-1 {
					m.cursor++
				}
			case tea.KeyPgUp:
				m.cursor -= m.visibleItems()
				if m.cursor < 0 {
					m.cursor = 0
				}
			case tea.KeyPgDown:
				m.cursor += m.visibleItems()
				if m.cursor >= len(m.filtered) {
					m.cursor = len(m.filtered) - 1
				}
				if m.cursor < 0 {
					m.cursor = 0
				}
			case tea.KeyHome:
				m.cursor = 0
			case tea.KeyEnd:
				if len(m.filtered) > 0 {
					m.cursor = len(m.filtered) - 1
				}
			}
			if m.cursor != prevCursor {
				return m, m.imageCmd()
			}
		}
	}

	return m, nil
}

// imageCmd returns a tea.Cmd that shows or clears the kitty image.
func (m *model) imageCmd() tea.Cmd {
	entry := m.selectedEntry()
	wasImage := m.imageShown

	if entry != nil && entry.IsImage {
		m.imageShown = true
		m.prevCursor = m.cursor
		e := *entry
		w := m.width
		h := m.height
		return func() tea.Msg {
			listWidth := w * 45 / 100
			previewCol := listWidth + 3
			previewRow := 5
			contentHeight := h - 4
			previewWidth := w - listWidth - 2
			if contentHeight < 1 {
				contentHeight = 1
			}
			showKittyImage(e, previewCol, previewRow, previewWidth, contentHeight)
			return imageShownMsg{}
		}
	}

	m.imageShown = false
	m.prevCursor = m.cursor
	if wasImage {
		return func() tea.Msg {
			clearKittyImages()
			return imageShownMsg{}
		}
	}
	return nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if !m.loaded {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			dimStyle().Render("Loading..."))
	}

	contentHeight := m.height - 4
	if contentHeight < 1 {
		contentHeight = 1
	}

	listWidth := m.width * 45 / 100
	previewWidth := m.width - listWidth - 2

	searchBar := m.renderSearchBar(m.width)
	listView := m.renderList(listWidth, contentHeight)
	previewView := m.renderPreview(previewWidth, contentHeight)

	// Pad preview text to full height so the border spans the entire panel
	previewView = padToHeight(previewView, contentHeight)
	previewRendered := previewBorderStyle().Width(previewWidth).Render(previewView)
	listView = clipLines(listView, contentHeight)
	previewRendered = clipLines(previewRendered, contentHeight)

	content := lipgloss.JoinHorizontal(lipgloss.Top, listView, previewRendered)

	return searchBar + "\n" + content
}

func padToHeight(s string, height int) string {
	lines := strings.Split(s, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func clipLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}

func (m model) renderSearchBar(width int) string {
	icon := dimStyle().Render("/ ")
	var text string
	if m.searchInput != "" {
		text = fgStyle().Render(m.searchInput) + accentStyle().Render("_")
	} else {
		text = dimStyle().Render("Type to search...")
	}
	return searchStyle().Width(width - 4).Render(icon + text)
}

func (m model) renderList(width, height int) string {
	if len(m.filtered) == 0 {
		return dimStyle().Width(width).Height(height).Render("  No entries")
	}

	visItems := m.visibleItems()
	start := 0
	if m.cursor >= visItems {
		start = m.cursor - visItems + 1
	}
	end := start + visItems
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	var lines []string
	for i := start; i < end; i++ {
		entry := m.filtered[i]
		pinned := m.meta.isPinned(entry.ID)
		ts := relativeTime(m.meta.timestamp(entry.ID))

		prefix := "  "
		if pinned {
			prefix = "* "
		}

		preview := entry.Preview
		if entry.IsImage {
			preview = "[image " + entry.ImageDim + "]"
		}

		maxLen := width - 4
		if maxLen < 10 {
			maxLen = 10
		}
		if len(preview) > maxLen {
			preview = preview[:maxLen-1] + "~"
		}

		line1 := prefix + preview
		line2 := "    " + ts

		if i == m.cursor {
			line1 = selectedStyle().Width(width).Render(line1)
			line2 = dimSelectedStyle().Width(width).Render(line2)
		} else {
			if pinned {
				line1 = accentStyle().Render(prefix) + fgStyle().Render(preview)
			} else {
				line1 = fgStyle().Width(width).Render(line1)
			}
			line2 = dimStyle().Width(width).Render(line2)
		}

		lines = append(lines, line1)
		if len(lines) < height {
			lines = append(lines, line2)
		}
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

func (m model) renderPreview(width, height int) string {
	entry := m.selectedEntry()
	if entry == nil {
		return dimStyle().Render("No selection")
	}

	if entry.IsImage {
		return dimStyle().Render("[" + entry.ImageDim + " " + entry.ImageFmt + "]")
	}

	return renderTextPreview(*entry, width, height)
}

func (m model) visibleItems() int {
	contentHeight := m.height - 4
	v := contentHeight / 2
	if v < 1 {
		return 1
	}
	return v
}
