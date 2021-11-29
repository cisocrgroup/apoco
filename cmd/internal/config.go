package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config defines the command's configuration.
type Config struct {
	Model    string              `json:"model,omitempty"`
	LM       map[string]LMConfig `json:"lm"`
	Profiler ProfilerConfig      `json:"profiler"`
	RR       TrainingConfig      `json:"rr"`
	DM       DMConfig            `json:"dm"`
	MS       TrainingConfig      `json:"ms"`
	FF       TrainingConfig      `json:"ff"`
	Nocr     int                 `json:"nocr"`
	Cache    bool                `json:"cache"`
	GT       bool                `json:"gt"`
	AlignLev bool                `json:"alignLev"`
}

// LMConfig configures the path to a language model csv file.
type LMConfig struct {
	Path string `json:"path"`
}

// ProfilerConfig holds the profiler's configuration values.
type ProfilerConfig struct {
	Exe    string `json:"exe"`
	Config string `json:"config"`
}

// TrainingConfig encloses different training settings.
type TrainingConfig struct {
	Features     []string `json:"features"`
	LearningRate float64  `json:"learningRate"`
	Ntrain       int      `json:"ntrain"`
}

// DMConfig encloses settings for dm training.
type DMConfig struct {
	TrainingConfig
	Filter string `json:"filter"` // cautious, courageous or redundant
}

// UpdateInConfig updates the value in dest with val if the according
// value is not the zero-type for the underlying type.  Dest must be a
// pointer type to either string, int, float64 or bool.  Otherwise the
// function panics.
func UpdateInConfig(dest, val interface{}) {
	switch dest.(type) {
	case *string:
		v := val.(string)
		if val != "" {
			(*dest.(*string)) = v
		}
	case *int:
		v := val.(int)
		if v != 0 {
			(*dest.(*int)) = v
		}
	case *float64:
		v := val.(float64)
		if v != 0 {
			(*dest.(*float64)) = v
		}
	case *bool:
		v := val.(bool)
		if v {
			(*dest.(*bool)) = v
		}
	default:
		panic("bad type")
	}
}

// ReadConfig reads the config from a json or toml file.  If
// the name is empty, an empty configuration file is returned.
// If name has the prefix '{' and the suffix '}' the name is
// interpreted as a json string and parsed accordingly (OCR-D compability).
func ReadConfig(name string) (*Config, error) {
	var config Config
	if name == "" {
		return &config, nil
	}
	if strings.HasPrefix(name, "{") && strings.HasSuffix(name, "}") {
		r := strings.NewReader(name)
		if err := json.NewDecoder(r).Decode(&config); err != nil {
			return nil, fmt.Errorf("readConfig %s: %v", name, err)
		}
		return &config, nil
	}
	is, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("readConfig %s: %v", name, err)
	}
	defer is.Close()
	if strings.HasSuffix(name, ".toml") {
		if _, err := toml.DecodeReader(is, &config); err != nil {
			return nil, fmt.Errorf("readConfig %s: %v", name, err)
		}
		return &config, nil
	}
	if err := json.NewDecoder(is).Decode(&config); err != nil {
		return nil, fmt.Errorf("readConfig %s: %v", name, err)
	}
	return &config, nil
}

const (
	Cautious   string = "cautious"
	Courageous string = "courageous"
	Redundant  string = "redundant"
)
