package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"
)

// renderTextPreview decodes and word-wraps text for the preview pane.
func renderTextPreview(entry ClipEntry, snippets *SnippetStore, width, height int) string {
	data, err := decodeEntry(entry, snippets)
	if err != nil {
		return dimStyle().Render("(decode error)")
	}

	text := string(data)
	if text == "" {
		return dimStyle().Render("(empty)")
	}

	// Word wrap at boundaries, then hard-break long unbreakable strings
	wrapped := wrap.String(wordwrap.String(text, width), width)

	lines := strings.Split(wrapped, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}

	return fgStyle().Render(strings.Join(lines, "\n"))
}

// showKittyImage writes kitty graphics protocol escape sequences directly
// to /dev/tty, bypassing bubbletea/lipgloss which would corrupt APC sequences.
// Kitty renders images on a separate layer above text, so they persist
// across bubbletea screen redraws.
func showKittyImage(entry ClipEntry, snippets *SnippetStore, col, row, cols, rows int) {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer tty.Close()

	data, err := decodeEntry(entry, snippets)
	if err != nil || len(data) == 0 {
		return
	}

	imgW, imgH := parseImageDim(entry.ImageDim)
	fitC, fitR := fitImage(imgW, imgH, cols, rows)

	offsetCol := (cols - fitC) / 2
	offsetRow := (rows - fitR) / 2

	b64 := base64.StdEncoding.EncodeToString(data)

	// Clear all previous images
	fmt.Fprint(tty, "\x1b_Ga=d,d=A\x1b\\")

	// Save cursor, move to centered position
	fmt.Fprintf(tty, "\x1b7\x1b[%d;%dH", row+offsetRow, col+offsetCol)

	// Transmit via direct data (t=d) with chunking for cross-terminal compat.
	// Use write-based chunking: split into 4096-byte base64 segments (aligned
	// to 4-byte groups) so each chunk is independently valid base64.
	const chunkSize = 4092 // 4 * 1023, safely under 4096 and aligned
	first := true
	for len(b64) > 0 {
		chunk := b64
		more := 0
		if len(chunk) > chunkSize {
			chunk = b64[:chunkSize]
			b64 = b64[chunkSize:]
			more = 1
		} else {
			b64 = ""
		}
		if first {
			fmt.Fprintf(tty, "\x1b_Ga=T,t=d,f=100,c=%d,r=%d,m=%d,q=2;%s\x1b\\",
				fitC, fitR, more, chunk)
			first = false
		} else {
			fmt.Fprintf(tty, "\x1b_Gm=%d,q=2;%s\x1b\\", more, chunk)
		}
	}

	// Restore cursor position
	fmt.Fprint(tty, "\x1b8")
}

// parseImageDim parses "988x606" into width, height. Returns 0,0 on failure.
func parseImageDim(dim string) (int, int) {
	var w, h int
	fmt.Sscanf(dim, "%dx%d", &w, &h)
	return w, h
}

// fitImage calculates cell cols/rows that fit within maxCols x maxRows
// while preserving the image aspect ratio. Cell aspect ratio is ~1:2
// (each cell is roughly twice as tall as it is wide in pixels).
func fitImage(imgW, imgH, maxCols, maxRows int) (int, int) {
	if imgW <= 0 || imgH <= 0 || maxCols <= 0 || maxRows <= 0 {
		return maxCols, maxRows
	}

	// Convert cell dimensions to a common pixel-like unit.
	// A cell is ~2x tall, so maxRows cells = maxRows*2 in "width units".
	areaW := float64(maxCols)
	areaH := float64(maxRows) * 2.0

	imgAspect := float64(imgW) / float64(imgH)

	// Fit within area preserving aspect ratio
	fitW := areaW
	fitH := fitW / imgAspect
	if fitH > areaH {
		fitH = areaH
		fitW = fitH * imgAspect
	}

	// Convert back to cell units
	cellCols := int(fitW)
	cellRows := int(fitH / 2.0)
	if cellCols < 1 {
		cellCols = 1
	}
	if cellRows < 1 {
		cellRows = 1
	}
	if cellCols > maxCols {
		cellCols = maxCols
	}
	if cellRows > maxRows {
		cellRows = maxRows
	}

	return cellCols, cellRows
}

// clearKittyImages removes all kitty graphics placements via /dev/tty.
func clearKittyImages() {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer tty.Close()
	fmt.Fprint(tty, "\x1b_Ga=d,d=A\x1b\\")
}

// kittyImageClear returns the escape sequence to clear images (for final cleanup on stdout).
func kittyImageClear() string {
	return "\x1b_Ga=d,d=A\x1b\\"
}
