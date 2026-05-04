package live

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/csweichel/go-pen/pkg/plot"
)

type interactiveState struct {
	Manifest plot.InteractiveManifest `json:"manifest"`
}

type interactiveUpdateRequest struct {
	Tool   string `json:"tool"`
	Region string `json:"region"`
}

func (s *server) currentInteractiveState(fn string) json.RawMessage {
	if fn == "" {
		return nil
	}

	s.ensureProfileLoaded(fn)

	s.mu.Lock()
	defer s.mu.Unlock()
	raw := s.interactiveStates[fn]
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func (s *server) saveInteractiveState(fn string, raw json.RawMessage) {
	s.mu.Lock()
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		delete(s.interactiveStates, fn)
	} else {
		s.interactiveStates[fn] = append(json.RawMessage(nil), raw...)
	}
	s.mu.Unlock()
	s.persistProfile(fn)
}

func (s *server) interactiveState(fn string) (interactiveState, error) {
	manifest, err := describeSketchInteractive(s.tmpdir, fn, s.customArgs, s.currentSketchArgs(fn), s.currentInteractiveState(fn))
	if err != nil {
		return interactiveState{}, err
	}
	return interactiveState{Manifest: manifest}, nil
}

func (s *server) updateInteractive(fn string, req interactiveUpdateRequest) (interactiveState, error) {
	if fn == "" {
		return interactiveState{}, fmt.Errorf("no sketch selected")
	}

	nextState, err := applySketchInteractive(s.tmpdir, fn, s.customArgs, s.currentSketchArgs(fn), s.currentInteractiveState(fn), plot.InteractiveAction{
		Tool:   req.Tool,
		Region: req.Region,
	})
	if err != nil {
		return interactiveState{}, err
	}

	s.saveInteractiveState(fn, nextState)
	return s.interactiveState(fn)
}

func describeSketchInteractive(tmpdir string, fn string, customArgs []string, currentArgs map[string]string, state json.RawMessage) (plot.InteractiveManifest, error) {
	manifest := plot.InteractiveManifest{
		Canvas:  plot.InteractiveCanvas{},
		Tools:   []plot.InteractiveTool{},
		Regions: []plot.InteractiveRegion{},
	}

	dir, base, err := resolveSketchEntrypoint(fn)
	if err != nil {
		return manifest, err
	}

	renderArgs := splitManagedRenderArgs(customArgs)
	remainingArgs, _ := stripSketchArgsFlags(renderArgs.RemainingArgs)
	remainingArgs, _ = stripManagedOptimiseFlags(remainingArgs, map[string]struct{}{
		"llo":   {},
		"vpype": {},
	})

	statePath, cleanup, err := writeInteractiveStateFile(tmpdir, state)
	if err != nil {
		return manifest, err
	}
	defer cleanup()

	cmdArgs := []string{"run", base, "--interactive-describe", "--interactive-state", statePath}
	if encoded := encodeSketchArgsValue(currentArgs); encoded != "" {
		cmdArgs = append(cmdArgs, "--args", encoded)
	}
	cmdArgs = append(cmdArgs, remainingArgs...)

	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return manifest, fmt.Errorf("go run failed: %w: %s", err, msg)
		}
		return manifest, fmt.Errorf("go run failed: %w", err)
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return manifest, nil
	}
	if err := json.Unmarshal([]byte(trimmed), &manifest); err != nil {
		return manifest, fmt.Errorf("cannot decode interactive manifest: %w", err)
	}
	if manifest.Tools == nil {
		manifest.Tools = []plot.InteractiveTool{}
	}
	if manifest.Regions == nil {
		manifest.Regions = []plot.InteractiveRegion{}
	}
	return manifest, nil
}

func applySketchInteractive(tmpdir string, fn string, customArgs []string, currentArgs map[string]string, state json.RawMessage, action plot.InteractiveAction) (json.RawMessage, error) {
	dir, base, err := resolveSketchEntrypoint(fn)
	if err != nil {
		return nil, err
	}

	renderArgs := splitManagedRenderArgs(customArgs)
	remainingArgs, _ := stripSketchArgsFlags(renderArgs.RemainingArgs)
	remainingArgs, _ = stripManagedOptimiseFlags(remainingArgs, map[string]struct{}{
		"llo":   {},
		"vpype": {},
	})

	statePath, cleanup, err := writeInteractiveStateFile(tmpdir, state)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	cmdArgs := []string{
		"run", base,
		"--interactive-apply",
		"--interactive-state", statePath,
		"--interactive-tool", action.Tool,
		"--interactive-region", action.Region,
	}
	if encoded := encodeSketchArgsValue(currentArgs); encoded != "" {
		cmdArgs = append(cmdArgs, "--args", encoded)
	}
	cmdArgs = append(cmdArgs, remainingArgs...)

	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return nil, fmt.Errorf("go run failed: %w: %s", err, msg)
		}
		return nil, fmt.Errorf("go run failed: %w", err)
	}

	var res plot.InteractiveApplyResult
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}
	if err := json.Unmarshal([]byte(trimmed), &res); err != nil {
		return nil, fmt.Errorf("cannot decode interactive apply result: %w", err)
	}
	if len(res.State) == 0 {
		return nil, nil
	}
	return append(json.RawMessage(nil), res.State...), nil
}

func writeInteractiveStateFile(tmpdir string, state json.RawMessage) (string, func(), error) {
	if len(state) == 0 {
		state = json.RawMessage("{}")
	}

	f, err := os.CreateTemp(tmpdir, "go-pen-interactive-*.json")
	if err != nil {
		return "", nil, err
	}

	name := f.Name()
	cleanup := func() {
		_ = os.Remove(name)
	}

	if _, err := f.Write(state); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return name, cleanup, nil
}
