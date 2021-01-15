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

var modelFlags = struct {
	histPats, ocrPats bool
}{}

func init() {
	modelCMD.Flags().BoolVarP(&modelFlags.histPats, "hist-pats", "p", false,
		"output global historical pattern probabilities")
	modelCMD.Flags().BoolVarP(&modelFlags.ocrPats, "ocr-pats", "e", false,
		"output global ocr error pattern probabilities")
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
		if modelFlags.histPats {
			printpats(name, "hist", model.GlobalHistPatterns)
		}
		if modelFlags.ocrPats {
			printpats(name, "ocr", model.GlobalOCRPatterns)
		}
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
				_, err := fmt.Printf("%s %s/%d %s(%d) %.13f\n",
					name, typ, nocr, data.Features[f], i+1, ws[f+i])
				chk(err)

			}
		}
	}
}

func printpats(name, typ string, pats map[string]float64) {
	for pat, prob := range pats {
		_, err := fmt.Printf("%s %s %s %.13f\n", name, typ, pat, prob)
		chk(err)
	}
}

func printjson(args []string) {
	var models []modelst
	for _, name := range args {
		model, err := apoco.ReadModel(name, "")
		chk(err)
		st := modelst{
			Name: name,
			Data: make(map[string][]feature),
		}
		if modelFlags.histPats {
			st.GlobalHistPatterns = model.GlobalHistPatterns
		}
		if modelFlags.ocrPats {
			st.GlobalOCRPatterns = model.GlobalOCRPatterns
		}
		for typ, data := range model.Models {
			st.Data[typ] = jsonfeatures(typ, data)
		}
		models = append(models, st)
	}
	chk(json.NewEncoder(os.Stdout).Encode(models))
}

func jsonfeatures(typ string, ds map[int]apoco.ModelData) []feature {
	var features []feature
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
				features = append(features, feature{
					Name:   data.Features[f],
					Nocr:   i + 1,
					Weight: ws[f+i],
				})
			}
		}
	}
	return features
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
	Name               string
	Data               map[string][]feature
	GlobalHistPatterns map[string]float64 `json:",omitempty"`
	GlobalOCRPatterns  map[string]float64 `json:",omitempty"`
}

type feature struct {
	Name   string
	Weight float64
	Nocr   int
}
