package apoco

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
	NoCache        bool     `json:"noCache"`
}

// ReadConfig reads the config from a json file.
func ReadConfig(file string) (*Config, error) {
	is, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("readConfig %s: %v", file, err)
	}
	defer is.Close()
	var config Config
	if strings.HasSuffix(file, ".toml") {
		if err := toml.DecoderReader(is, &config); err != nil {
			return nil, fmt.Errorf("readConfig %s: %v", file, err)
		}
		return &config, nil
	}
	if err := json.NewDecoder(is).Decode(&config); err != nil {
		return nil, fmt.Errorf("readConfig %s: %v", file, err)
	}
	return &config, nil
}

// Overwrite overwrites the appropriate variables in the config file
// with the given values.  Values only overwrite the variables if they
// are not go's default zero value.
func (c *Config) Overwrite(model string, nocr int, nocache bool) {
	if model != "" {
		c.Model = model
	}
	if nocr != 0 {
		c.Nocr = nocr
	}
	if nocache {
		c.NoCache = nocache
	}
}
