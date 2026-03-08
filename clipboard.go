package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// cliphist format: [[ binary data 169 KiB png 988x606 ]]
var imagePattern = regexp.MustCompile(`^\[\[ binary data .+ (\w+) (\d+x\d+) \]\]$`)

type ClipEntry struct {
	ID       string // numeric cliphist ID
	RawLine  string // full line from cliphist list (id\tpreview)
	Preview  string // preview text (after tab)
	IsImage  bool
	ImageFmt string // e.g. "png", "jpg"
	ImageDim string // e.g. "840x265"
}

func cliphistList() ([]ClipEntry, error) {
	out, err := exec.Command("cliphist", "list").Output()
	if err != nil {
		return nil, fmt.Errorf("cliphist list: %w", err)
	}

	var entries []ClipEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		entry := parseLine(line)
		entries = append(entries, entry)
	}
	return entries, nil
}

func parseLine(line string) ClipEntry {
	e := ClipEntry{RawLine: line}

	idx := strings.IndexByte(line, '\t')
	if idx > 0 {
		e.ID = line[:idx]
		e.Preview = line[idx+1:]
	} else {
		e.ID = line
		e.Preview = line
	}

	if m := imagePattern.FindStringSubmatch(e.Preview); m != nil {
		e.IsImage = true
		e.ImageFmt = m[1]
		e.ImageDim = m[2]
	}

	return e
}

func cliphistDecode(rawLine string) ([]byte, error) {
	cmd := exec.Command("cliphist", "decode")
	cmd.Stdin = strings.NewReader(rawLine)
	return cmd.Output()
}

func cliphistDelete(rawLine string) error {
	cmd := exec.Command("cliphist", "delete")
	cmd.Stdin = strings.NewReader(rawLine)
	return cmd.Run()
}
