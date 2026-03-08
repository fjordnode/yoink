package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FlexTime handles both RFC3339 strings and unix float timestamps from jq.
type FlexTime struct {
	time.Time
}

func (ft *FlexTime) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err == nil {
			ft.Time = t
			return nil
		}
		t, err = time.Parse(time.RFC3339, s)
		if err == nil {
			ft.Time = t
			return nil
		}
	}
	// Try as unix float
	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		sec := int64(f)
		nsec := int64((f - float64(sec)) * 1e9)
		ft.Time = time.Unix(sec, nsec)
		return nil
	}
	ft.Time = time.Time{}
	return nil
}

func (ft FlexTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(ft.Time.Format(time.RFC3339Nano))
}

type EntryMeta struct {
	Pinned    bool     `json:"pinned"`
	Timestamp FlexTime `json:"timestamp"`
	Source    string   `json:"source,omitempty"`
}

type MetadataStore struct {
	Entries map[string]EntryMeta `json:"entries"`
	path    string
}

func metadataPath() string {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "yoink", "metadata.json")
}

func loadMetadata() *MetadataStore {
	p := metadataPath()
	store := &MetadataStore{
		Entries: make(map[string]EntryMeta),
		path:    p,
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return store
	}
	_ = json.Unmarshal(data, store)
	if store.Entries == nil {
		store.Entries = make(map[string]EntryMeta)
	}
	return store
}

func (s *MetadataStore) reconcile(ids []string) {
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	// Remove stale entries
	for id := range s.Entries {
		if !idSet[id] {
			delete(s.Entries, id)
		}
	}

	// Add new entries
	for _, id := range ids {
		if _, ok := s.Entries[id]; !ok {
			s.Entries[id] = EntryMeta{Timestamp: FlexTime{time.Now()}}
		}
	}
}

func (s *MetadataStore) togglePin(id string) {
	meta := s.Entries[id]
	meta.Pinned = !meta.Pinned
	s.Entries[id] = meta
}

func (s *MetadataStore) isPinned(id string) bool {
	return s.Entries[id].Pinned
}

func (s *MetadataStore) timestamp(id string) time.Time {
	return s.Entries[id].Timestamp.Time
}

func (s *MetadataStore) source(id string) string {
	return s.Entries[id].Source
}

func (s *MetadataStore) remove(id string) {
	delete(s.Entries, id)
}

func (s *MetadataStore) save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d"
		}
		return fmt.Sprintf("%dd", days)
	}
}
