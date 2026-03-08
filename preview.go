package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// renderTextPreview decodes and word-wraps text for the preview pane.
func renderTextPreview(entry ClipEntry, width, height int) string {
	data, err := cliphistDecode(entry.RawLine)
	if err != nil {
		return dimStyle().Render("(decode error)")
	}

	text := string(data)
	if text == "" {
		return dimStyle().Render("(empty)")
	}

	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if len(line) <= width {
			lines = append(lines, line)
		} else {
			for len(line) > width {
				lines = append(lines, line[:width])
				line = line[width:]
			}
			if line != "" {
				lines = append(lines, line)
			}
		}
		if len(lines) >= height {
			break
		}
	}

	if len(lines) > height {
		lines = lines[:height]
	}

	return fgStyle().Render(strings.Join(lines, "\n"))
}

// showKittyImage writes kitty graphics protocol escape sequences directly
// to /dev/tty, bypassing bubbletea/lipgloss which would corrupt APC sequences.
// Kitty renders images on a separate layer above text, so they persist
// across bubbletea screen redraws.
func showKittyImage(entry ClipEntry, col, row, cols, rows int) {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer tty.Close()

	// Decode image data from cliphist
	data, err := cliphistDecode(entry.RawLine)
	if err != nil || len(data) == 0 {
		return
	}

	// Write to temp file for kitty to read
	tmp, err := os.CreateTemp("", "clip-img-*.png")
	if err != nil {
		return
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return
	}
	tmp.Close()

	// Parse image dimensions and fit to cell area preserving aspect ratio.
	// Terminal cells are ~2x taller than wide, so 1 cell row ≈ 2 cell cols.
	imgW, imgH := parseImageDim(entry.ImageDim)
	fitC, fitR := fitImage(imgW, imgH, cols, rows)

	pathB64 := base64.StdEncoding.EncodeToString([]byte(tmpPath))

	// Clear all previous images
	fmt.Fprint(tty, "\x1b_Ga=d,d=A\x1b\\")

	// Save cursor, move to preview pane position
	fmt.Fprintf(tty, "\x1b7\x1b[%d;%dH", row, col)

	// Transmit and display image
	// a=T: transmit+display, t=f: file path, f=100: auto format
	// c/r: cell columns/rows to fit, q=2: quiet (suppress responses)
	fmt.Fprintf(tty, "\x1b_Ga=T,t=f,f=100,c=%d,r=%d,q=2;%s\x1b\\", fitC, fitR, pathB64)

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
