package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type SnippetMeta struct {
	ID        string    `json:"id"`
	Preview   string    `json:"preview"`
	IsImage   bool      `json:"is_image,omitempty"`
	ImageFmt  string    `json:"image_fmt,omitempty"`
	ImageDim  string    `json:"image_dim,omitempty"`
	ImageSize string    `json:"image_size,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source,omitempty"`
}

type SnippetStore struct {
	Snippets []SnippetMeta `json:"snippets"`
	NextID   int           `json:"next_id"`
	dir      string
}

func snippetsDir() string {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "yoink", "snippets")
}

func loadSnippets() *SnippetStore {
	dir := snippetsDir()
	store := &SnippetStore{dir: dir, NextID: 1}

	data, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		return store
	}
	_ = json.Unmarshal(data, store)
	if store.NextID < 1 {
		store.NextID = 1
	}
	return store
}

func (s *SnippetStore) save(e ClipEntry, data []byte, source string) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}

	id := fmt.Sprintf("%d", s.NextID)
	s.NextID++

	dataPath := filepath.Join(s.dir, id+".dat")
	if err := os.WriteFile(dataPath, data, 0o644); err != nil {
		return err
	}

	s.Snippets = append(s.Snippets, SnippetMeta{
		ID:        id,
		Preview:   e.Preview,
		IsImage:   e.IsImage,
		ImageFmt:  e.ImageFmt,
		ImageDim:  e.ImageDim,
		ImageSize: e.ImageSize,
		Timestamp: time.Now(),
		Source:    source,
	})

	return s.flush()
}

func (s *SnippetStore) remove(id string) error {
	os.Remove(filepath.Join(s.dir, id+".dat"))

	filtered := s.Snippets[:0]
	for _, sn := range s.Snippets {
		if sn.ID != id {
			filtered = append(filtered, sn)
		}
	}
	s.Snippets = filtered
	return s.flush()
}

func (s *SnippetStore) readData(id string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.dir, id+".dat"))
}

func (s *SnippetStore) toClipEntries() []ClipEntry {
	entries := make([]ClipEntry, len(s.Snippets))
	for i, sn := range s.Snippets {
		entries[i] = ClipEntry{
			ID:          "snippet:" + sn.ID,
			Preview:     sn.Preview,
			IsImage:     sn.IsImage,
			ImageFmt:    sn.ImageFmt,
			ImageDim:    sn.ImageDim,
			ImageSize:   sn.ImageSize,
			IsSaved:     true,
			SnippetID:   sn.ID,
			SavedSource: sn.Source,
			SavedTime:   sn.Timestamp,
		}
	}
	return entries
}

func (s *SnippetStore) flush() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, "index.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, "index.json"))
}

func (s *SnippetStore) isSaved(preview string, isImage bool) bool {
	for _, sn := range s.Snippets {
		if sn.Preview == preview && sn.IsImage == isImage {
			return true
		}
	}
	return false
}
