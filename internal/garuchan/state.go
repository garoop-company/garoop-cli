package garuchan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	Name          string    `json:"name"`
	Model         string    `json:"model"`
	BornAt        time.Time `json:"born_at"`
	LastFedAt     time.Time `json:"last_fed_at"`
	TotalFeeds    int       `json:"total_feeds"`
	TotalCalories int       `json:"total_calories"`
	Stage         string    `json:"stage"`
}

func Birth(path string, name string, model string, force bool) (*State, error) {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil, fmt.Errorf("既に誕生済みです: %s", path)
		}
	}
	now := time.Now()
	s := &State{
		Name:          name,
		Model:         model,
		BornAt:        now,
		LastFedAt:     now,
		TotalFeeds:    0,
		TotalCalories: 0,
		Stage:         "newborn",
	}
	if err := Save(path, s); err != nil {
		return nil, err
	}
	return s, nil
}

func Load(path string) (*State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func Save(path string, s *State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func Feed(s *State, calories int) {
	if calories < 1 {
		calories = 1
	}
	s.TotalFeeds++
	s.TotalCalories += calories
	s.LastFedAt = time.Now()
	s.Stage = resolveStage(s.TotalCalories)
}

func resolveStage(totalCalories int) string {
	switch {
	case totalCalories >= 600:
		return "toddler"
	case totalCalories >= 300:
		return "crawler"
	case totalCalories >= 120:
		return "smiling"
	default:
		return "newborn"
	}
}
