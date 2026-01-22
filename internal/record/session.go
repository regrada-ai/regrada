package record

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Session struct {
	ID        string    `json:"id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	TraceIDs  []string  `json:"trace_ids"`
}

func NewSession(id string) *Session {
	if id == "" {
		id = time.Now().UTC().Format("20060102-150405")
	}
	return &Session{ID: id, StartTime: time.Now().UTC()}
}

func (s *Session) AddTrace(id string) {
	if id == "" {
		return
	}
	s.TraceIDs = append(s.TraceIDs, id)
}

func (s *Session) Finalize() {
	s.EndTime = time.Now().UTC()
	s.TraceIDs = uniqueSorted(s.TraceIDs)
}

func SaveSession(dir string, s *Session) (string, error) {
	if s == nil {
		return "", fmt.Errorf("session is nil")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s.json", s.ID))
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, data, 0644)
}

func LoadSession(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func LatestSession(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var latest string
	var latestTime time.Time
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latest = filepath.Join(dir, entry.Name())
		}
	}
	if latest == "" {
		return "", fmt.Errorf("no session files found in %s", dir)
	}
	return latest, nil
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
