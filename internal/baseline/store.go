package baseline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/matias/regrada/internal/git"
	"github.com/matias/regrada/internal/util"
)

type Baseline struct {
	CaseID      string          `json:"case_id"`
	BaselineKey string          `json:"baseline_key"`
	Provider    string          `json:"provider"`
	Model       string          `json:"model"`
	ParamsHash  string          `json:"params_hash"`
	Aggregates  Aggregates      `json:"aggregates"`
	GoldenText  string          `json:"golden_text,omitempty"`
	GoldenJSON  json.RawMessage `json:"golden_json,omitempty"`
}

type Aggregates struct {
	PassRate      float64 `json:"pass_rate"`
	LatencyP95MS  int     `json:"latency_p95_ms"`
	RefusalRate   float64 `json:"refusal_rate"`
	JSONValidRate float64 `json:"json_valid_rate"`
}

type Store interface {
	Load(caseID, baselineKey string) (Baseline, error)
	Save(caseID, baselineKey string, b Baseline) error
}

type LocalStore struct {
	baseDir string
}

func NewLocalStore(baseDir string) *LocalStore {
	return &LocalStore{baseDir: baseDir}
}

func (s *LocalStore) Load(caseID, baselineKey string) (Baseline, error) {
	path := filepath.Join(s.baseDir, caseID, baselineKey+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Baseline{}, err
	}
	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return Baseline{}, err
	}
	return b, nil
}

func (s *LocalStore) Save(caseID, baselineKey string, b Baseline) error {
	path := filepath.Join(s.baseDir, caseID, baselineKey+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := util.CanonicalJSON(b)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

type GitStore struct {
	ref     string
	baseDir string
	client  git.Client
}

func NewGitStore(ref, baseDir string, client git.Client) *GitStore {
	return &GitStore{ref: ref, baseDir: baseDir, client: client}
}

func (s *GitStore) Load(caseID, baselineKey string) (Baseline, error) {
	path := filepath.ToSlash(filepath.Join(s.baseDir, caseID, baselineKey+".json"))
	data, err := s.client.ShowFile(s.ref, path)
	if err != nil {
		return Baseline{}, err
	}
	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return Baseline{}, fmt.Errorf("parse baseline: %w", err)
	}
	return b, nil
}

func (s *GitStore) Save(caseID, baselineKey string, b Baseline) error {
	return fmt.Errorf("git baseline store is read-only")
}
