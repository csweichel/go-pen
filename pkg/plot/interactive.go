package plot

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type InteractiveTool struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type InteractiveCanvas struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type InteractiveRegion struct {
	ID        string `json:"id"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Label     string `json:"label,omitempty"`
	Hint      string `json:"hint,omitempty"`
	Stroke    string `json:"stroke,omitempty"`
	Fill      string `json:"fill,omitempty"`
	TextColor string `json:"textColor,omitempty"`
}

type InteractiveManifest struct {
	Canvas      InteractiveCanvas   `json:"canvas"`
	Tools       []InteractiveTool   `json:"tools"`
	Regions     []InteractiveRegion `json:"regions"`
	DefaultTool string              `json:"defaultTool,omitempty"`
}

type InteractiveAction struct {
	Tool   string `json:"tool"`
	Region string `json:"region"`
}

type InteractiveApplyResult struct {
	State json.RawMessage `json:"state,omitempty"`
}

type InteractiveHooks struct {
	Describe func(p Canvas, args map[string]string, state json.RawMessage) (InteractiveManifest, error)
	Apply    func(p Canvas, args map[string]string, state json.RawMessage, action InteractiveAction) (json.RawMessage, error)
}

var interactiveStateContext struct {
	mu   sync.RWMutex
	path string
}

func setInteractiveStatePath(path string) {
	interactiveStateContext.mu.Lock()
	defer interactiveStateContext.mu.Unlock()
	interactiveStateContext.path = path
}

func interactiveStatePath() string {
	interactiveStateContext.mu.RLock()
	defer interactiveStateContext.mu.RUnlock()
	return interactiveStateContext.path
}

func InteractiveStateRaw() (json.RawMessage, error) {
	path := interactiveStatePath()
	if path == "" {
		return nil, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	raw = json.RawMessage(raw)
	if len(raw) == 0 {
		return nil, nil
	}
	return raw, nil
}

func DecodeInteractiveState[T any]() (T, error) {
	var zero T

	raw, err := InteractiveStateRaw()
	if err != nil {
		return zero, err
	}
	if len(raw) == 0 {
		return zero, nil
	}

	var state T
	if err := json.Unmarshal(raw, &state); err != nil {
		return zero, fmt.Errorf("cannot decode interactive state: %w", err)
	}
	return state, nil
}
