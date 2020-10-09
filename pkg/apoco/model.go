package apoco

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
)

// Model holds the different models for the different number of OCRs.
type Model struct {
	Models map[string]map[int]modelData `json:"models"`
	Ngrams FreqList                     `json:"ngrams"`
}

type modelData struct {
	Features []string
	Model    *ml.LR
}

// ReadModel reads a model from a gzip compressed input file.  If the
// given file does not exist, an empty model is returned.  If the
// model does not contain a valid ngram frequency list, the list is
// loaded from the given path.
func ReadModel(model, ngrams string) (Model, error) {
	log.Printf("reading model from %s", model)
	in, err := os.Open(model)
	if os.IsNotExist(err) {
		m := Model{Models: make(map[string]map[int]modelData)}
		if err := m.readGzippedNgrams(ngrams); err != nil {
			return Model{}, fmt.Errorf("readModel %s: %s", model, err)
		}
		return m, nil
	}
	if err != nil {
		return Model{}, fmt.Errorf("readModel %s: %s", model, err)
	}
	defer in.Close()
	zip, err := gzip.NewReader(in)
	if err != nil {
		return Model{}, fmt.Errorf("readModel %s: %s", model, err)
	}
	defer zip.Close()
	var m Model
	if err := json.NewDecoder(zip).Decode(&m); err != nil {
		return Model{}, fmt.Errorf("readModel %s: %s", model, err)
	}
	if m.Ngrams.FreqList != nil {
		return m, nil
	}
	if err := m.readGzippedNgrams(ngrams); err != nil {
		return Model{}, fmt.Errorf("readModel %s: %s", model, err)
	}
	return m, nil
}

func (m *Model) readGzippedNgrams(path string) error {
	log.Printf("reading ngrams from %s", path)
	is, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("readGzippedNGrams %s: %v", path, err)
	}
	defer is.Close()
	gz, err := gzip.NewReader(is)
	if err != nil {
		return fmt.Errorf("readGzippedNGrams %s: %v", path, err)
	}
	defer gz.Close()
	if err := m.Ngrams.loadCSV(gz); err != nil {
		return fmt.Errorf("readGzippedNGrams: %s: %v", path, err)
	}
	return nil
}

// Write writes the model as json encoded, gziped file to the given
// path overwriting any previous existing models.
func (m Model) Write(path string) (err error) {
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("write %s: %v", path, err)
	}
	defer func() {
		if exx := out.Close(); exx != nil && err == nil {
			err = fmt.Errorf("write %s: %v", path, err)
		}
	}()
	zip := gzip.NewWriter(out)
	defer func() {
		if exx := zip.Close(); exx != nil && err == nil {
			err = fmt.Errorf("write %s: %v", path, err)
		}
	}()
	if err := json.NewEncoder(zip).Encode(m); err != nil {
		return fmt.Errorf("write %s: %v", path, err)
	}
	return nil
}

// Put inserts the weights and the according feature set for the given
// configuration into this model.
func (m Model) Put(mod string, nocr int, lr *ml.LR, fs []string) {
	if _, ok := m.Models[mod]; !ok {
		m.Models[mod] = make(map[int]modelData)
	}
	m.Models[mod][nocr] = modelData{
		Features: fs,
		Model:    lr,
	}
}

// Get loads the the model and the according feature set for the given configuration.
func (m Model) Get(mod string, nocr int) (*ml.LR, FeatureSet, error) {
	if _, ok := m.Models[mod]; !ok {
		return nil, nil, fmt.Errorf("load: cannot find: %s/%d", mod, nocr)
	}
	if _, ok := m.Models[mod][nocr]; !ok {
		return nil, nil, fmt.Errorf("load: cannot find: %s/%d", mod, nocr)
	}
	fs, err := NewFeatureSet(m.Models[mod][nocr].Features...)
	if err != nil {
		return nil, nil, fmt.Errorf("load: %v", err)
	}
	return m.Models[mod][nocr].Model, fs, nil
}
