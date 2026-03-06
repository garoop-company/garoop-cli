package parenting

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Entry struct {
	Time    time.Time `json:"time"`
	Child   string    `json:"child"`
	Kind    string    `json:"kind"`
	Memo    string    `json:"memo"`
	Minutes int       `json:"minutes"`
	Amount  float64   `json:"amount"`
}

type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() ([]Entry, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, err
	}
	var out []Entry
	if len(data) == 0 {
		return []Entry{}, nil
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) Append(e Entry) error {
	all, err := s.Load()
	if err != nil {
		return err
	}
	all = append(all, e)
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o644)
}

func ValidateKind(kind string) error {
	switch strings.ToLower(kind) {
	case "meal", "sleep", "study", "play", "health":
		return nil
	default:
		return fmt.Errorf("kindは meal/sleep/study/play/health のいずれかを指定してください")
	}
}

func FilterSince(entries []Entry, since time.Time) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.Time.After(since) || e.Time.Equal(since) {
			out = append(out, e)
		}
	}
	return out
}
