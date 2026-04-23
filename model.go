package main

import (
	"fmt"
	"os"
	"os/exec"
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
	snippets     *SnippetStore
	source       SourceWindow
	cursor       int
	prevCursor   int
	searchInput  string
	width        int
	height       int
	pendingPaste string
	status       string
	imageShown   bool
	fullPreview bool // full-screen image preview mode
	loaded      bool // whether initial load has completed
	pendingExec bool // external process launching, block further execs
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
		queryRunes := []rune(strings.ToLower(m.searchInput))
		m.filtered = nil
		m.matchIndices = nil
		for _, e := range m.entries {
			displayText := e.Preview
			if e.IsImage {
				displayText = "🖼️  [IMAGE] " + e.ImageDim
			}
			lowerRunes := []rune(strings.ToLower(displayText))
			idx := runeIndex(lowerRunes, queryRunes)
			if idx < 0 {
				continue
			}
			m.filtered = append(m.filtered, e)
			// Build match indices for the found substring (rune-based)
			indices := make([]int, len(queryRunes))
			for j := range indices {
				indices[j] = idx + j
			}
			m.matchIndices = append(m.matchIndices, indices)
		}
	}

	// Saved entries first, preserve order within groups
	type entryWithIdx struct {
		entry   ClipEntry
		indices []int
	}
	var savedItems, regularItems []entryWithIdx
	for i, e := range m.filtered {
		var indices []int
		if i < len(m.matchIndices) {
			indices = m.matchIndices[i]
		}
		item := entryWithIdx{entry: e, indices: indices}
		if e.IsSaved {
			savedItems = append(savedItems, item)
		} else {
			regularItems = append(regularItems, item)
		}
	}
	all := append(savedItems, regularItems...)
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

	case editorClosedMsg:
		return m, tea.Batch(
			loadEntriesCmd,
			tea.Tick(300*time.Millisecond, func(_ time.Time) tea.Msg {
				return execCooldownMsg{}
			}),
		)

	case execCooldownMsg:
		m.pendingExec = false
		return m, nil

	case entriesLoadedMsg:
		// Merge new entries without disrupting user's position
		selectedID := ""
		if e := m.selectedEntry(); e != nil {
			selectedID = e.ID
		}

		clipEntries := msg.entries
		if clipEntries == nil {
			clipEntries = []ClipEntry{}
		}
		m.snippets = loadSnippets()
		m.entries = append(m.snippets.toClipEntries(), clipEntries...)
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
		} else {
			for i, e := range m.filtered {
				if !e.IsSaved {
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
				runes := []rune(m.searchInput)
				m.searchInput = string(runes[:len(runes)-1])
				m.applyFilter()
				m.prevCursor = -1
				return m, m.imageCmd()
			}
			return m, nil
		}
		if msg.Type == tea.KeyRunes {
			m.searchInput += string(msg.Runes)
			m.applyFilter()
			m.prevCursor = -1
			return m, m.imageCmd()
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

		case keyPaste:
			if e := m.selectedEntry(); e != nil {
				if e.IsSaved {
					data, err := m.snippets.readData(e.SnippetID)
					if err == nil {
						cmd := execWlCopy(data)
						if cmd.Run() == nil {
							clearKittyImages()
							m.pendingPaste = "__snippet__"
							return m, tea.Quit
						}
					}
				} else {
					clearKittyImages()
					m.pendingPaste = e.RawLine
					return m, tea.Quit
				}
			}

		case keyCopy:
			if e := m.selectedEntry(); e != nil {
				data, err := decodeEntry(*e, m.snippets)
				if err == nil {
					cmd := execWlCopy(data)
					if cmd.Run() == nil {
						clearKittyImages()
						return m, tea.Quit
					}
				}
			}

		case keyDelete:
			if e := m.selectedEntry(); e != nil {
				var deleted bool
				if e.IsSaved {
					if err := m.snippets.remove(e.SnippetID); err == nil {
						deleted = true
					}
				} else {
					if err := cliphistDelete(e.RawLine); err == nil {
						m.meta.remove(e.ID)
						_ = m.meta.save()
						deleted = true
					}
				}
				if deleted {
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
			if e := m.selectedEntry(); e != nil && !m.pendingExec {
				if e.IsImage {
					m.fullPreview = true
					m.prevCursor = -1
					return m, m.imageCmd()
				}
				m.pendingExec = true
				return m, openInEditor(*e, m.snippets)
			}

		case keySave:
			if e := m.selectedEntry(); e != nil {
				if e.IsSaved {
					if err := m.snippets.remove(e.SnippetID); err == nil {
						m.status = "Unsaved"
						return m, tea.Batch(
							tea.Tick(statusDuration, func(_ time.Time) tea.Msg {
								return statusClearMsg{}
							}),
							loadEntriesCmd,
						)
					}
				} else {
					data, err := decodeEntry(*e, m.snippets)
					if err == nil {
						src := m.meta.source(e.ID)
						if m.snippets.save(*e, data, src) == nil {
							m.status = "Saved ★"
							return m, tea.Batch(
								tea.Tick(statusDuration, func(_ time.Time) tea.Msg {
									return statusClearMsg{}
								}),
								loadEntriesCmd,
							)
						}
					}
				}
			}

		case keyEditImage:
			if e := m.selectedEntry(); e != nil && e.IsImage && !m.pendingExec {
				m.pendingExec = true
				return m, openInSatty(*e, m.snippets)
			}

		case keyRefresh:
			return m, loadEntriesCmd

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
		snips := m.snippets
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
			showKittyImage(e, snips, col, row, cols, rows)
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
		hint := dimStyle().Render("↑↓ cycle · Enter paste · Ctrl+N copy · any key to close")
		return "\x1b[?25l" + lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Bottom, hint)
	}

	// Search box(3) + hint bar(1) + content
	contentHeight := m.height - 4
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

	hints := lipgloss.Place(m.width, 1, lipgloss.Center, lipgloss.Center,
		veryDimStyle().Render("Enter paste · ^N copy · ^P save · ^D del · Tab expand · ^E annotate · Esc quit"))

	return searchBar + content + "\n" + hints
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
	inner := prompt + text
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Color8)).
		Width(width - 2).
		Render(inner)
	return box + "\n"
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

		preview := entry.Preview
		if entry.IsImage {
			preview = "🖼️  [IMAGE] " + entry.ImageDim
		}

		maxLen := width - 4
		if maxLen < 10 {
			maxLen = 10
		}
		previewRunes := []rune(preview)
		if len(previewRunes) > maxLen {
			preview = string(previewRunes[:maxLen-1]) + "…"
		}

		highlighted := highlightMatches(preview, m.matchIndices[i], entry.IsImage)

		indicator := "  "
		if entry.IsSaved {
			indicator = accentStyle().Render("★ ")
		}

		if i == m.cursor {
			selBg := selectedRowStyle(width)
			selInd := "  "
			if entry.IsSaved {
				selInd = selectedPinStyle().Render("★ ")
			}
			lines = append(lines, selBg.Render(selInd+selectedHighlight(preview, m.matchIndices[i], entry.IsImage)))
		} else {
			lines = append(lines, indicator+highlighted)
		}

		// Dim separator between entries
		if i < end-1 && len(lines) < height {
			sep := veryDimStyle().Render("  " + strings.Repeat("─", width-4))
			lines = append(lines, sep)
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
// selectedHighlight renders text on the accent-background selection bar.
func selectedHighlight(text string, indices []int, isImage bool) string {
	base := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Background)).
		Background(lipgloss.Color(theme.Accent))
	bold := base.Bold(true)

	if len(indices) == 0 || isImage {
		return bold.Render(text)
	}

	matchSet := make(map[int]bool, len(indices))
	for _, idx := range indices {
		matchSet[idx] = true
	}

	var result strings.Builder
	for i, r := range []rune(text) {
		ch := string(r)
		if matchSet[i] {
			result.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(theme.SelectionForeground)).
				Background(lipgloss.Color(theme.Accent)).
				Bold(true).Underline(true).Render(ch))
		} else {
			result.WriteString(base.Render(ch))
		}
	}
	return result.String()
}

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
		var src string
		if entry.IsSaved {
			src = entry.SavedSource
		} else {
			src = m.meta.source(entry.ID)
		}
		if src != "" {
			label += " · " + src
		}
		return dimStyle().Render(label)
	}

	return renderTextPreview(*entry, m.snippets, width, height)
}

// runeIndex finds the first occurrence of needle in haystack (rune slices).
func runeIndex(haystack, needle []rune) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func (m model) visibleItems() int {
	contentHeight := m.height - 4 // search box(3) + hint bar(1)
	// 2 lines per entry: content + separator
	v := (contentHeight + 1) / 2
	if v < 1 {
		return 1
	}
	return v
}

// openInEditor decodes a clipboard entry to a temp file and opens it in nvim (readonly).
func openInEditor(e ClipEntry, snippets *SnippetStore) tea.Cmd {
	data, err := decodeEntry(e, snippets)
	if err != nil || len(data) == 0 {
		return nil
	}

	tmp, err := os.CreateTemp("", "yoink-*.txt")
	if err != nil {
		return nil
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil
	}
	tmp.Close()

	c := exec.Command("nvim", "-R",
		"+set nomodifiable",
		"+autocmd VimEnter * nnoremap <buffer> <Esc> :q!<CR>",
		tmp.Name())
	return tea.ExecProcess(c, func(err error) tea.Msg {
		os.Remove(tmp.Name())
		return editorClosedMsg{}
	})
}

type editorClosedMsg struct{}
type execCooldownMsg struct{}

func openInSatty(e ClipEntry, snippets *SnippetStore) tea.Cmd {
	data, err := decodeEntry(e, snippets)
	if err != nil || len(data) == 0 {
		return nil
	}

	tmp, err := os.CreateTemp("", "yoink-*.png")
	if err != nil {
		return nil
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil
	}
	tmp.Close()

	c := exec.Command("sh", "-c",
		fmt.Sprintf("satty --filename '%s' --early-exit 2>/dev/null", tmp.Name()))
	return tea.ExecProcess(c, func(err error) tea.Msg {
		os.Remove(tmp.Name())
		return editorClosedMsg{}
	})
}
