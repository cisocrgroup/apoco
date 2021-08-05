package apoco

import (
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"os"

	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
)

// Model holds the different models for the different number of OCRs.
type Model struct {
	Models             map[string]map[int]ModelData
	GlobalHistPatterns map[string]float64
	GlobalOCRPatterns  map[string]float64
	Ngrams             *FreqList
}

// ModelData holds a linear regression model.
type ModelData struct {
	Features []string
	Model    *ml.LR
}

// ReadModel reads a model from a gob compressed input file.  If the
// given file does not exist, an empty model is returned.  If the
// model does not contain a valid ngram frequency list, the list is
// loaded from the given path.
func ReadModel(model, ngrams string) (Model, error) {
	Log("reading model from %s", model)
	in, err := os.Open(model)
	if os.IsNotExist(err) {
		m := Model{Models: make(map[string]map[int]ModelData)}
		if err := m.readGzippedNgrams(ngrams); err != nil {
			return Model{}, fmt.Errorf("read model %s: %s", model, err)
		}
		return m, nil
	}
	if err != nil {
		return Model{}, fmt.Errorf("read model %s: %s", model, err)
	}
	defer in.Close()
	var m Model
	if err := gob.NewDecoder(in).Decode(&m); err != nil {
		return Model{}, fmt.Errorf("read model %s: %s", model, err)
	}
	Log("read model from %s", model)
	return m, nil
}

func (m *Model) readGzippedNgrams(name string) error {
	if name == "" { // Do not load the trigram model.
		return nil
	}
	Log("reading ngrams from %s", name)
	is, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("readGzippedNGrams %s: %v", name, err)
	}
	defer is.Close()
	gz, err := gzip.NewReader(is)
	if err != nil {
		return fmt.Errorf("readGzippedNGrams %s: %v", name, err)
	}
	defer gz.Close()
	m.Ngrams = &FreqList{}
	if err := m.Ngrams.loadCSV(gz); err != nil {
		return fmt.Errorf("readGzippedNGrams: %s: %v", name, err)
	}
	return nil
}

// Write writes the model as gob encoded, gziped file to the given
// path overwriting any previous existing models.
func (m Model) Write(name string) (err error) {
	out, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("write %s: %v", name, err)
	}
	defer func() {
		if exx := out.Close(); exx != nil && err == nil {
			err = fmt.Errorf("write %s: %v", name, err)
		}
	}()
	if err := gob.NewEncoder(out).Encode(m); err != nil {
		return fmt.Errorf("write %s: %v", name, err)
	}
	return nil
}

// Put inserts the weights and the according feature set for the given
// configuration into this model.
func (m Model) Put(mod string, nocr int, lr *ml.LR, fs []string) {
	if _, ok := m.Models[mod]; !ok {
		m.Models[mod] = make(map[int]ModelData)
	}
	m.Models[mod][nocr] = ModelData{
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
