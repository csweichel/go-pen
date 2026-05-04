package plot

import (
	"fmt"
	"strconv"
	"strings"
)

type ArgType string

const (
	ArgTypeInt   ArgType = "int"
	ArgTypeFloat ArgType = "float"
	ArgTypeBool  ArgType = "bool"
)

type ArgSpec struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Type        ArgType  `json:"type"`
	Default     *string  `json:"default,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
}

type RunOpt func(*RunConfig)

type RunConfig struct {
	ArgsSchema []ArgSpec
}

func WithArgSchema(specs ...ArgSpec) RunOpt {
	return func(cfg *RunConfig) {
		cfg.ArgsSchema = append([]ArgSpec(nil), specs...)
	}
}

func WithArgsSchema(specs ...ArgSpec) RunOpt {
	return WithArgSchema(specs...)
}

func IntArg(name string, description string, min int, max int, def int) ArgSpec {
	minf := float64(min)
	maxf := float64(max)
	if def < min {
		def = min
	}
	if def > max {
		def = max
	}
	defv := strconv.Itoa(def)
	return ArgSpec{
		Name:        name,
		Description: description,
		Type:        ArgTypeInt,
		Default:     &defv,
		Min:         &minf,
		Max:         &maxf,
	}
}

func FloatArg(name string, description string, min float64, max float64, def float64) ArgSpec {
	minf := min
	maxf := max
	if def < min {
		def = min
	}
	if def > max {
		def = max
	}
	defv := strconv.FormatFloat(def, 'f', -1, 64)
	return ArgSpec{
		Name:        name,
		Description: description,
		Type:        ArgTypeFloat,
		Default:     &defv,
		Min:         &minf,
		Max:         &maxf,
	}
}

func BoolArg(name string, description string, def bool) ArgSpec {
	defv := strconv.FormatBool(def)
	return ArgSpec{
		Name:        name,
		Description: description,
		Type:        ArgTypeBool,
		Default:     &defv,
	}
}

func (s ArgSpec) Normalize(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if s.Default != nil {
			return *s.Default, nil
		}
		return "", nil
	}

	switch s.Type {
	case ArgTypeInt:
		v, err := strconv.Atoi(raw)
		if err != nil {
			return "", fmt.Errorf("must be an integer")
		}
		if s.Min != nil && v < int(*s.Min) {
			v = int(*s.Min)
		}
		if s.Max != nil && v > int(*s.Max) {
			v = int(*s.Max)
		}
		return strconv.Itoa(v), nil
	case ArgTypeFloat:
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return "", fmt.Errorf("must be a number")
		}
		if s.Min != nil && v < *s.Min {
			v = *s.Min
		}
		if s.Max != nil && v > *s.Max {
			v = *s.Max
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case ArgTypeBool:
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return "", fmt.Errorf("must be true or false")
		}
		return strconv.FormatBool(v), nil
	default:
		return "", fmt.Errorf("unsupported arg type %q", s.Type)
	}
}
