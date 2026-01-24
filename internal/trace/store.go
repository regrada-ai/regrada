package trace

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store interface {
	Append(t Trace) error
	List(filter TraceFilter) ([]Trace, error)
	Read(id string) (Trace, error)
}

type TraceFilter struct {
	IDs   []string
	Since *time.Time
	Until *time.Time
	Limit int
}

type LocalStore struct {
	baseDir string
	mu      sync.Mutex
}

func NewLocalStore(baseDir string) *LocalStore {
	return &LocalStore{baseDir: baseDir}
}

func (s *LocalStore) Append(t Trace) error {
	if t.Timestamp.IsZero() {
		t.Timestamp = time.Now().UTC()
	}
	dateDir := filepath.Join(s.baseDir, t.Timestamp.Format("2006-01-02"))
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dateDir, "traces.jsonl")

	line, err := json.Marshal(t)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *LocalStore) List(filter TraceFilter) ([]Trace, error) {
	var traces []Trace
	if _, err := os.Stat(s.baseDir); err != nil {
		return nil, err
	}

	ids := make(map[string]bool)
	for _, id := range filter.IDs {
		ids[id] = true
	}

	err := filepath.WalkDir(s.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "traces.jsonl") {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var t Trace
			if err := json.Unmarshal(scanner.Bytes(), &t); err != nil {
				return fmt.Errorf("parse trace: %w", err)
			}
			if len(ids) > 0 && !ids[t.TraceID] {
				continue
			}
			if filter.Since != nil && t.Timestamp.Before(*filter.Since) {
				continue
			}
			if filter.Until != nil && t.Timestamp.After(*filter.Until) {
				continue
			}
			traces = append(traces, t)
			if filter.Limit > 0 && len(traces) >= filter.Limit {
				return nil
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(traces, func(i, j int) bool {
		return traces[i].Timestamp.Before(traces[j].Timestamp)
	})

	return traces, nil
}

func (s *LocalStore) Read(id string) (Trace, error) {
	traces, err := s.List(TraceFilter{IDs: []string{id}, Limit: 1})
	if err != nil {
		return Trace{}, err
	}
	if len(traces) == 0 {
		return Trace{}, fmt.Errorf("trace %s not found", id)
	}
	return traces[0], nil
}
