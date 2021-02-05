package apoco

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config defines the command's configuration.
type Config struct {
	Model          string   `json:"model,omitempty"`
	Ngrams         string   `json:"ngrams"`
	ProfilerBin    string   `json:"profilerBin"`
	ProfilerConfig string   `json:"profilerConfig"`
	RRFeatures     []string `json:"rrFeatures"`
	DMFeatures     []string `json:"dmFeatures"`
	LearningRate   float64  `json:"learningRate"`
	Ntrain         int      `json:"ntrain"`
	Nocr           int      `json:"nocr"`
	Cache          bool     `json:"cache"`
	Cautious       bool     `json:"cautious"`
}

// ReadConfig reads the config from a json or toml file.
func ReadConfig(name string) (*Config, error) {
	is, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("readConfig %s: %v", name, err)
	}
	defer is.Close()
	var config Config
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
func (c *Config) Overwrite(model string, nocr int, cautious, cache bool) {
	if model != "" {
		c.Model = model
	}
	if nocr != 0 {
		c.Nocr = nocr
	}
	if cautious {
		c.Cautious = cautious
	}
	if cache {
		c.Cache = cache
	}
}
