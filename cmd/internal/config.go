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
	Model          string           `json:"model,omitempty"`
	Ngrams         string           `json:"ngrams"`
	ProfilerBin    string           `json:"profilerBin"`
	ProfilerConfig string           `json:"profilerConfig"`
	RR             TrainingSettings `json:"rr"`
	DM             DMSettings       `json:"dm"`
	Nocr           int              `json:"nocr"`
	Cache          bool             `json:"cache"`
	GT             bool             `json:"gt"`
}

// TrainingSettings encloses different training settings.
type TrainingSettings struct {
	Features     []string `json:"features"`
	LearningRate float64  `json:"learningRate"`
	Ntrain       int      `json:"ntrain"`
}

// DMSettings encloses settings for dm training.
type DMSettings struct {
	TrainingSettings
	FilterInstances string // cautious, courageous or redundant
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

// Overwrite overwrites the appropriate variables in the config file
// with the given values.  Values only overwrite the variables if they
// are not go's default zero value.
func (c *Config) Overwrite(model, filterInstances string, nocr int, cache, gt bool) {
	if model != "" {
		c.Model = model
	}
	if filterInstances != "" {
		c.DM.FilterInstances = filterInstances
	}
	if nocr != 0 {
		c.Nocr = nocr
	}
	if cache {
		c.Cache = cache
	}
	if gt {
		c.GT = gt
	}
}
