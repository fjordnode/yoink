package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	entries      []ClipEntry
	filtered     []ClipEntry
	matchIndices [][]int // fuzzy match character indices per filtered entry
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
	fullPreview  bool // full-screen image preview mode
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
		m.matchIndices = make([][]int, len(m.entries))
	} else {
		query := strings.ToLower(m.searchInput)
		m.filtered = nil
		m.matchIndices = nil
		for _, e := range m.entries {
			displayText := e.Preview
			if e.IsImage {
				displayText = "[IMAGE] " + e.ImageDim
			}
			lower := strings.ToLower(displayText)
			idx := strings.Index(lower, query)
			if idx < 0 {
				continue
			}
			m.filtered = append(m.filtered, e)
			// Build match indices for the found substring
			indices := make([]int, len(query))
			for j := range indices {
				indices[j] = idx + j
			}
			m.matchIndices = append(m.matchIndices, indices)
		}
	}

	// Pinned first, preserve order within groups
	type entryWithIdx struct {
		entry   ClipEntry
		indices []int
	}
	var pinnedItems, unpinnedItems []entryWithIdx
	for i, e := range m.filtered {
		var indices []int
		if i < len(m.matchIndices) {
			indices = m.matchIndices[i]
		}
		item := entryWithIdx{entry: e, indices: indices}
		if m.meta.isPinned(e.ID) {
			pinnedItems = append(pinnedItems, item)
		} else {
			unpinnedItems = append(unpinnedItems, item)
		}
	}
	all := append(pinnedItems, unpinnedItems...)
	m.filtered = make([]ClipEntry, len(all))
	m.matchIndices = make([][]int, len(all))
	for i, item := range all {
		m.filtered[i] = item.entry
		m.matchIndices[i] = item.indices
	}

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
		// Full preview mode: arrows cycle images, Enter pastes, other keys exit
		if m.fullPreview {
			if classifyKey(msg) == keyPaste {
				if e := m.selectedEntry(); e != nil {
					clearKittyImages()
					m.pendingPaste = e.RawLine
					return m, tea.Quit
				}
			}
			if msg.Type == tea.KeyUp || msg.Type == tea.KeyDown {
				dir := 1
				if msg.Type == tea.KeyUp {
					dir = -1
				}
				// Find next image entry in direction
				for i := m.cursor + dir; i >= 0 && i < len(m.filtered); i += dir {
					if m.filtered[i].IsImage {
						m.cursor = i
						m.prevCursor = -1
						return m, m.imageCmd()
					}
				}
				return m, nil
			}
			m.fullPreview = false
			m.prevCursor = -1
			return m, m.imageCmd()
		}

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

		case keyFullPreview:
			if e := m.selectedEntry(); e != nil && e.IsImage {
				m.fullPreview = true
				m.prevCursor = -1
				return m, m.imageCmd()
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
		full := m.fullPreview
		return func() tea.Msg {
			var col, row, cols, rows int
			if full {
				col = 2
				row = 2
				cols = w - 2
				rows = h - 2
			} else {
				listWidth := w * 50 / 100
				col = listWidth + 3
				row = 4 // search(1) + blank(1) + newline(1) + 1-based
				cols = w - listWidth - 2
				rows = h - 3
			}
			if rows < 1 {
				rows = 1
			}
			showKittyImage(e, col, row, cols, rows)
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
			dimStyle().Render("Loading…"))
	}

	if m.fullPreview {
		hint := dimStyle().Render("↑↓ cycle · Enter to paste · any key to close")
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Bottom, hint)
	}

	// Search prompt(1) + blank line(1) + content
	contentHeight := m.height - 3
	if contentHeight < 1 {
		contentHeight = 1
	}

	listWidth := m.width * 50 / 100
	previewWidth := m.width - listWidth - 2

	searchBar := m.renderSearchBar(m.width)
	listView := m.renderList(listWidth, contentHeight)
	previewView := m.renderPreview(previewWidth, contentHeight)

	previewView = padToHeight(previewView, contentHeight)
	previewRendered := previewBorderStyle().Width(previewWidth).Render(previewView)
	listView = clipLines(listView, contentHeight)
	previewRendered = clipLines(previewRendered, contentHeight)

	content := lipgloss.JoinHorizontal(lipgloss.Top, listView, previewRendered)

	return searchBar + content
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
	prompt := accentStyle().Render("> ")
	var text string
	if m.searchInput != "" {
		text = fgStyle().Render(m.searchInput) + accentStyle().Render("_")
		count := len(m.filtered)
		text += dimStyle().Render(fmt.Sprintf(" %d results", count))
	} else {
		text = dimStyle().Render("Type to search…")
	}
	return " " + prompt + text + "\n\n"
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
		if src := m.meta.source(entry.ID); src != "" {
			ts += " · " + src
		}

		preview := entry.Preview
		if entry.IsImage {
			preview = "[IMAGE] " + entry.ImageDim
		}

		maxLen := width - 4
		if maxLen < 10 {
			maxLen = 10
		}
		if len(preview) > maxLen {
			preview = preview[:maxLen-1] + "…"
		}

		// Highlight fuzzy match characters
		highlighted := highlightMatches(preview, m.matchIndices[i], entry.IsImage)

		if i == m.cursor {
			bar := accentStyle().Render("▏")
			line1 := bar + " " + highlighted
			line2 := bar + "   " + veryDimStyle().Render(ts)
			lines = append(lines, line1)
			if len(lines) < height {
				lines = append(lines, line2)
			}
		} else {
			pin := "  "
			if pinned {
				pin = accentStyle().Render("● ")
			}
			line1 := pin + highlighted
			line2 := "    " + veryDimStyle().Render(ts)
			lines = append(lines, line1)
			if len(lines) < height {
				lines = append(lines, line2)
			}
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

// highlightMatches renders a preview string with fuzzy match indices in accent color.
// If no indices, renders plain. For images, the indices refer to the original preview
// text, not the "Image WxH" replacement, so we skip highlighting for images.
func highlightMatches(text string, indices []int, isImage bool) string {
	if len(indices) == 0 || isImage {
		return fgStyle().Render(text)
	}

	matchSet := make(map[int]bool, len(indices))
	for _, idx := range indices {
		matchSet[idx] = true
	}

	var result strings.Builder
	runes := []rune(text)
	for i, r := range runes {
		ch := string(r)
		if matchSet[i] {
			result.WriteString(accentStyle().Bold(true).Render(ch))
		} else {
			result.WriteString(fgStyle().Render(ch))
		}
	}
	return result.String()
}

func (m model) renderPreview(width, height int) string {
	entry := m.selectedEntry()
	if entry == nil {
		return dimStyle().Render("No selection")
	}

	if entry.IsImage {
		label := entry.ImageFmt + " · " + entry.ImageDim + " · " + entry.ImageSize
		if src := m.meta.source(entry.ID); src != "" {
			label += " · " + src
		}
		return dimStyle().Render(label)
	}

	return renderTextPreview(*entry, width, height)
}

func (m model) visibleItems() int {
	contentHeight := m.height - 3
	v := contentHeight / 2 // 2 lines per entry (text + timestamp)
	if v < 1 {
		return 1
	}
	return v
}
