package print

import (
	"encoding/json"
	"fmt"
	"os"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/finkf/gofiler"
	"github.com/spf13/cobra"
)

// modelCMD runs the apoco print model command.
var modelCMD = &cobra.Command{
	Use:   "model [MODEL...]",
	Short: "Print information about models",
	Run:   runModel,
}

func runModel(_ *cobra.Command, args []string) {
	if flags.json {
		printjson(args)
	} else {
		printmodels(args)

	}
}

func printmodels(args []string) {
	for _, name := range args {
		model, err := apoco.ReadModel(name, "")
		chk(err)
		for typ, data := range model.Models {
			printmodel(name, typ, data)
		}
	}
}

func printmodel(name, typ string, ds map[int]apoco.ModelData) {
	for nocr, data := range ds {
		ws := data.Model.Weights()
		fs, err := apoco.NewFeatureSet(data.Features...)
		chk(err)
		for f := range fs {
			for i := 0; i < nocr; i++ {
				_, ok := fs[f](mktok(typ, nocr), i, nocr)
				if !ok {
					continue
				}
				_, err := fmt.Printf("%s %s/%d %s(%d) %f\n",
					name, typ, nocr, data.Features[f], i+1, ws[f+i])
				chk(err)

			}
		}
	}
}

func printjson(args []string) {
	var models []modelst
	for _, name := range args {
		model, err := apoco.ReadModel(name, "")
		chk(err)
		for typ, data := range model.Models {
			models = append(models, jsonmodels(name, typ, data)...)
		}
	}
	chk(json.NewEncoder(os.Stdout).Encode(models))
}

func jsonmodels(name, typ string, ds map[int]apoco.ModelData) []modelst {
	var models []modelst
	for nocr, data := range ds {
		m := modelst{
			Name:         name,
			Type:         typ,
			Nocr:         nocr,
			LearningRate: data.Model.LearningRate,
			Ntrain:       data.Model.Ntrain,
		}
		ws := data.Model.Weights()
		fs, err := apoco.NewFeatureSet(data.Features...)
		chk(err)
		for f := range fs {
			for i := 0; i < nocr; i++ {
				_, ok := fs[f](mktok(typ, nocr), i, nocr)
				if !ok {
					continue
				}
				m.Features = append(m.Features, feature{
					Name:   data.Features[f],
					Nocr:   i + 1,
					Weight: ws[f+i],
				})
			}
		}
		models = append(models, m)
	}
	return models
}

func mktok(typ string, nocr int) apoco.T {
	switch typ {
	case "dm":
		return apoco.T{
			Tokens: make([]string, nocr),
			Payload: []apoco.Ranking{
				apoco.Ranking{Candidate: new(gofiler.Candidate)},
			},
		}
	default:
		return apoco.T{
			Tokens:  make([]string, nocr),
			Payload: new(gofiler.Candidate),
		}
	}
}

type modelst struct {
	Name, Type   string
	Features     []feature
	LearningRate float64
	Nocr, Ntrain int
}

type feature struct {
	Name   string
	Weight float64
	Nocr   int
}
