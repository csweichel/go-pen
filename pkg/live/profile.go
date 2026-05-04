package live

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

type sketchProfileFile struct {
	Version     int               `json:"version"`
	Args        map[string]string `json:"args,omitempty"`
	Interactive json.RawMessage   `json:"interactive,omitempty"`
}

func (s *server) ensureProfileLoaded(fn string) {
	if fn == "" {
		return
	}

	s.mu.Lock()
	if s.profileLoaded[fn] {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	path := sketchProfilePath(fn)
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.WithError(err).WithField("profile", path).Warn("cannot read sketch live profile")
		}
		s.mu.Lock()
		s.profileLoaded[fn] = true
		s.mu.Unlock()
		return
	}

	var profile sketchProfileFile
	if err := json.Unmarshal(raw, &profile); err != nil {
		log.WithError(err).WithField("profile", path).Warn("cannot decode sketch live profile")
		s.mu.Lock()
		s.profileLoaded[fn] = true
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.profileLoaded[fn] {
		return
	}
	s.profileLoaded[fn] = true

	if len(profile.Args) > 0 {
		argProfile := s.argProfiles[fn]
		argProfile.Values = copyStringMap(profile.Args)
		s.argProfiles[fn] = argProfile
	}
	if len(profile.Interactive) > 0 {
		s.interactiveStates[fn] = append(json.RawMessage(nil), profile.Interactive...)
	}
}

func (s *server) persistProfile(fn string) {
	if fn == "" {
		return
	}

	s.mu.Lock()
	profile := sketchProfileFile{Version: 1}
	if argProfile, ok := s.argProfiles[fn]; ok && len(argProfile.Values) > 0 {
		profile.Args = copyStringMap(argProfile.Values)
	}
	if raw := s.interactiveStates[fn]; len(raw) > 0 {
		profile.Interactive = append(json.RawMessage(nil), raw...)
	}
	s.profileLoaded[fn] = true
	s.mu.Unlock()

	path := sketchProfilePath(fn)
	if len(profile.Args) == 0 && len(profile.Interactive) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.WithError(err).WithField("profile", path).Warn("cannot remove empty sketch live profile")
		}
		return
	}

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		log.WithError(err).WithField("profile", path).Warn("cannot encode sketch live profile")
		return
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.WithError(err).WithField("profile", path).Warn("cannot write sketch live profile")
	}
}

func sketchProfilePath(fn string) string {
	if fn == "" {
		return ""
	}

	base := filepath.Base(fn)
	dir := filepath.Dir(fn)
	if strings.EqualFold(base, "main.go") {
		return filepath.Join(dir, ".go-pen-live.json")
	}

	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "" {
		name = "sketch"
	}
	return filepath.Join(dir, "."+name+".go-pen-live.json")
}
