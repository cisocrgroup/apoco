package internal

import (
	"git.sr.ht/~flobar/apoco/pkg/apoco"
)

// Aliases for Model holds the different models for the different training
// runs for a different number of OCRs.  It is used to save and load
// the models for the automatic postcorrection.
type (
	Model     = apoco.Model
	ModelData = apoco.ModelData
)

// ReadModel reads a model from a gob compressed input file.  If the
// given file does not exist, the according language models are loaded
// and a new model is returned.  If create is set to false no new
// model will be created and the model must be read from an existing
// file.
func ReadModel(name string, lms map[string]apoco.LMConfig, create bool) (*Model, error) {
	return apoco.ReadModel(name, lms, create)
}
