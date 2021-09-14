package internal

import (
	"bufio"
	"compress/gzip"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
)

// Model holds the different models for the different number of OCRs.
type Model struct {
	Models             map[string]map[int]ModelData
	GlobalHistPatterns map[string]float64
	GlobalOCRPatterns  map[string]float64
	LM                 map[string]*apoco.FreqList
}

// ModelData holds a linear regression model.
type ModelData struct {
	Features []string
	Model    *ml.LR
}

// ReadModel reads a model from a gob compressed input file.  If the
// given file does not exist, the according language models are loaded
// and a new model is returned.  If create is set to false no new
// model will be created and the model must be read from an existing
// file.
func ReadModel(name string, lms map[string]LMConfig, create bool) (*Model, error) {
	apoco.Log("reading model from %s", name)
	fail := func(err error) (*Model, error) {
		return nil, fmt.Errorf("read model %s: %v", name, err)
	}
	r, err := os.Open(name)
	// Create a new empty model file if it does not already exist and create=true.
	if create && os.IsNotExist(err) {
		lms, err := readLMs(lms)
		if err != nil {
			return fail(err)
		}
		return &Model{
			Models: make(map[string]map[int]ModelData),
			LM:     lms,
		}, nil
	}
	if err != nil {
		return fail(err)
	}
	defer r.Close()
	var model Model
	if err := gob.NewDecoder(r).Decode(&model); err != nil {
		return fail(err)
	}
	apoco.Log("read model from %s", name)
	return &model, nil
}

// Write writes the model as gob encoded, gziped file to the given
// path overwriting any previous existing models.
func (m *Model) Write(name string) (err error) {
	w, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("write %s: %v", name, err)
	}
	defer func() {
		if exx := w.Close(); exx != nil && err == nil {
			err = fmt.Errorf("write %s: %v", name, err)
		}
	}()
	if err := gob.NewEncoder(w).Encode(m); err != nil {
		return fmt.Errorf("write %s: %v", name, err)
	}
	return nil
}

// Put inserts the weights and the according feature set for the given
// configuration into this model.
func (m *Model) Put(mod string, nocr int, lr *ml.LR, fs []string) {
	if _, ok := m.Models[mod]; !ok {
		m.Models[mod] = make(map[int]ModelData)
	}
	m.Models[mod][nocr] = ModelData{
		Features: fs,
		Model:    lr,
	}
}

// Get loads the the model and the according feature set for the given
// configuration.
func (m *Model) Get(mod string, nocr int) (*ml.LR, apoco.FeatureSet, error) {
	fail := func(err error) (*ml.LR, apoco.FeatureSet, error) {
		return nil, nil, fmt.Errorf("get %s/%d: %v", mod, nocr, err)
	}
	if _, ok := m.Models[mod]; !ok {
		return fail(errors.New("cannot find"))
	}
	if _, ok := m.Models[mod][nocr]; !ok {
		return fail(errors.New("cannot find"))
	}
	fs, err := apoco.NewFeatureSet(m.Models[mod][nocr].Features...)
	if err != nil {
		return fail(err)
	}
	return m.Models[mod][nocr].Model, fs, nil
}

// readLMs read the frequency lists from the given CSV files.  The
// format of the file must be `n,str`.  If the name has the suffix
// `.gz`, a gzipped CSV file is assumed.
func readLMs(lms map[string]LMConfig) (map[string]*apoco.FreqList, error) {
	fail := func(err error) (map[string]*apoco.FreqList, error) {
		return nil, fmt.Errorf("read language models: %v", err)
	}
	if len(lms) == 0 {
		return nil, nil
	}
	ret := make(map[string]*apoco.FreqList)
	for name, conf := range lms {
		apoco.Log("reading language model %q from %s", name, conf.Path)
		lm, err := readLM(conf.Path)
		if err != nil {
			return fail(err)
		}
		ret[name] = lm
	}
	return ret, nil
}

func readLM(name string) (*apoco.FreqList, error) {
	fail := func(err error) (*apoco.FreqList, error) {
		return nil, fmt.Errorf("read language model %s: %v", name, err)
	}
	r, err := os.Open(name)
	if err != nil {
		return fail(err)
	}
	defer r.Close()
	var rr io.Reader = r
	if strings.HasSuffix(name, ".gz") {
		gzr, err := gzip.NewReader(r)
		if err != nil {
			return fail(err)
		}
		defer gzr.Close()
		rr = gzr
	}
	lm, err := readLMFromReader(rr)
	if err != nil {
		return fail(err)
	}
	return lm, nil
}

func readLMFromReader(r io.Reader) (*apoco.FreqList, error) {
	fail := func(n int, err error) (*apoco.FreqList, error) {
		return nil, fmt.Errorf("read at line %d: %v", n, err)
	}
	lm := apoco.FreqList{FreqList: make(map[string]int)}
	s := bufio.NewScanner(r)
	line := 0
	for s.Scan() {
		line++
		var n int
		var str string
		if _, err := fmt.Sscanf(s.Text(), "%d,%s", &n, &str); err != nil {
			return fail(line, err)
		}
		lm.FreqList[str] = n
		lm.Total += n
	}
	if err := s.Err(); err != nil {
		return fail(line, err)
	}
	return &lm, nil
}
