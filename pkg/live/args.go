package live

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/csweichel/go-pen/pkg/plot"
	log "github.com/sirupsen/logrus"
)

type argProfile struct {
	Specs         []plot.ArgSpec
	Values        map[string]string
	SourceModTime time.Time
	SchemaLoaded  bool
}

type argState struct {
	Specs  []plot.ArgSpec    `json:"specs"`
	Values map[string]string `json:"values,omitempty"`
	Raw    string            `json:"raw,omitempty"`
}

type argUpdateRequest struct {
	Values map[string]string `json:"values"`
	Raw    *string           `json:"raw"`
	Reset  bool              `json:"reset"`
}

func (s *server) currentSketchArgs(fn string) map[string]string {
	if fn == "" {
		return nil
	}

	profile := s.syncArgProfile(fn)
	return currentArgsForProfile(profile, s.customArgs)
}

func (s *server) argsState(fn string) argState {
	if fn == "" {
		return argState{Specs: []plot.ArgSpec{}}
	}

	profile := s.syncArgProfile(fn)
	return newArgState(profile.Specs, currentArgsForProfile(profile, s.customArgs))
}

func (s *server) updateArgs(fn string, req argUpdateRequest) (argState, error) {
	if fn == "" {
		return argState{}, fmt.Errorf("no sketch selected")
	}

	profile := s.syncArgProfile(fn)
	if req.Reset {
		profile.Values = nil
		s.saveArgProfile(fn, profile)
		return newArgState(profile.Specs, currentArgsForProfile(profile, s.customArgs)), nil
	}

	state := newArgState(profile.Specs, currentArgsForProfile(profile, s.customArgs))
	typedValues := copyStringMap(state.Values)
	if req.Values != nil {
		for k, v := range req.Values {
			typedValues[k] = v
		}
	}

	raw := state.Raw
	if req.Raw != nil {
		raw = *req.Raw
	}

	values, err := buildAuthoritativeArgs(profile.Specs, typedValues, raw)
	if err != nil {
		return argState{}, err
	}

	profile.Values = values
	s.saveArgProfile(fn, profile)
	return newArgState(profile.Specs, currentArgsForProfile(profile, s.customArgs)), nil
}

func (s *server) saveArgProfile(fn string, profile argProfile) {
	s.mu.Lock()
	s.argProfiles[fn] = profile
	s.mu.Unlock()
	s.persistProfile(fn)
}

func (s *server) syncArgProfile(fn string) argProfile {
	if fn == "" {
		return argProfile{}
	}

	s.ensureProfileLoaded(fn)

	modTime := sourceModTime(fn)

	s.mu.Lock()
	profile := s.argProfiles[fn]
	s.mu.Unlock()

	needsReload := !profile.SchemaLoaded || (!modTime.IsZero() && modTime.After(profile.SourceModTime))
	if needsReload {
		specs, err := describeSketchArgs(fn, s.customArgs, currentArgsForProfile(profile, s.customArgs))
		if err != nil {
			log.WithError(err).WithField("sketch", sketchNameForMainFile(fn)).Warn("cannot load sketch arg schema")
		} else {
			profile.Specs = specs
			profile.SchemaLoaded = true
			profile.SourceModTime = modTime
		}
	}

	s.saveArgProfile(fn, profile)
	return profile
}

func currentArgsForProfile(profile argProfile, customArgs []string) map[string]string {
	if profile.Values != nil {
		return copyStringMap(profile.Values)
	}
	return baseSketchArgs(customArgs)
}

func baseSketchArgs(customArgs []string) map[string]string {
	renderArgs := splitManagedRenderArgs(customArgs)
	_, sketchArgs := stripSketchArgsFlags(renderArgs.RemainingArgs)
	delete(sketchArgs, "debug")
	return sketchArgs
}

func newArgState(specs []plot.ArgSpec, currentArgs map[string]string) argState {
	known := make(map[string]struct{}, len(specs))
	values := make(map[string]string, len(specs))
	for _, spec := range specs {
		known[spec.Name] = struct{}{}
		raw := ""
		if currentArgs != nil {
			raw = currentArgs[spec.Name]
		}
		normalized, err := spec.Normalize(raw)
		if err != nil || normalized == "" {
			continue
		}
		values[spec.Name] = normalized
	}

	extras := make(map[string]string)
	for k, v := range currentArgs {
		if k == "debug" {
			continue
		}
		if _, ok := known[k]; ok {
			continue
		}
		extras[k] = v
	}

	return argState{
		Specs:  cloneArgSpecs(specs),
		Values: values,
		Raw:    encodeSketchArgsValue(extras),
	}
}

func buildAuthoritativeArgs(specs []plot.ArgSpec, typedValues map[string]string, raw string) (map[string]string, error) {
	values, err := parseSketchArgsValue(raw)
	if err != nil {
		return nil, err
	}
	if values == nil {
		values = make(map[string]string)
	}
	delete(values, "debug")

	for _, spec := range specs {
		input := ""
		if typedValues != nil {
			input = typedValues[spec.Name]
		}
		normalized, err := spec.Normalize(input)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", spec.Name, err)
		}
		if shouldPersistArgValue(spec, normalized) {
			values[spec.Name] = normalized
		} else {
			delete(values, spec.Name)
		}
	}

	return values, nil
}

func shouldPersistArgValue(spec plot.ArgSpec, value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	if spec.Default != nil && value == *spec.Default {
		return false
	}
	return true
}

func stripSketchArgsFlags(args []string) (remainingArgs []string, sketchArgs map[string]string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--args":
			if i+1 >= len(args) {
				remainingArgs = append(remainingArgs, a)
				continue
			}
			parsed, err := parseSketchArgsValue(args[i+1])
			if err != nil {
				remainingArgs = append(remainingArgs, a, args[i+1])
			} else {
				sketchArgs = mergeStringMaps(sketchArgs, parsed)
			}
			i++
		case strings.HasPrefix(a, "--args="):
			parsed, err := parseSketchArgsValue(strings.TrimPrefix(a, "--args="))
			if err != nil {
				remainingArgs = append(remainingArgs, a)
			} else {
				sketchArgs = mergeStringMaps(sketchArgs, parsed)
			}
		default:
			remainingArgs = append(remainingArgs, a)
		}
	}

	if len(sketchArgs) == 0 {
		return remainingArgs, nil
	}
	return remainingArgs, sketchArgs
}

func parseSketchArgsValue(raw string) (map[string]string, error) {
	res := make(map[string]string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return res, nil
	}

	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		key, value, ok := strings.Cut(part, "=")
		if !ok {
			key = part
			value = "true"
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, fmt.Errorf("invalid arg %q", part)
		}
		res[key] = value
	}

	return res, nil
}

func encodeSketchArgsValue(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		if strings.TrimSpace(k) == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, values[k]))
	}
	return strings.Join(parts, ",")
}

func mergeStringMaps(base map[string]string, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}

	res := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		res[k] = v
	}
	for k, v := range override {
		res[k] = v
	}
	return res
}

func copyStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	res := make(map[string]string, len(in))
	for k, v := range in {
		res[k] = v
	}
	return res
}

func cloneArgSpecs(specs []plot.ArgSpec) []plot.ArgSpec {
	if specs == nil {
		return []plot.ArgSpec{}
	}
	res := make([]plot.ArgSpec, len(specs))
	copy(res, specs)
	return res
}

func sourceModTime(fn string) time.Time {
	info, err := os.Stat(fn)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func describeSketchArgs(fn string, customArgs []string, currentArgs map[string]string) ([]plot.ArgSpec, error) {
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

	cmdArgs := []string{"run", base, "--args-schema"}
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

	var specs []plot.ArgSpec
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return []plot.ArgSpec{}, nil
	}
	if err := json.Unmarshal([]byte(trimmed), &specs); err != nil {
		return nil, fmt.Errorf("cannot decode args schema: %w", err)
	}
	if specs == nil {
		specs = []plot.ArgSpec{}
	}
	return specs, nil
}

func resolveSketchEntrypoint(fn string) (dir string, base string, err error) {
	stat, err := os.Stat(fn)
	if err != nil {
		return "", "", err
	}
	if stat.IsDir() {
		return fn, "main.go", nil
	}
	return filepath.Dir(fn), filepath.Base(fn), nil
}
