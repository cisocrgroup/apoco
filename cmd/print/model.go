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
	histPats, ocrPats, noWeights bool
}{}

func init() {
	modelCMD.Flags().BoolVarP(&modelFlags.histPats, "histpats", "p", false,
		"output global historical pattern probabilities")
	modelCMD.Flags().BoolVarP(&modelFlags.ocrPats, "ocrpats", "e", false,
		"output global ocr error pattern probabilities")
	modelCMD.Flags().BoolVarP(&modelFlags.noWeights, "noweights", "n", false,
		"do not output feature weights")
}

func runModel(_ *cobra.Command, args []string) {
	for _, name := range args {
		model, err := apoco.ReadModel(name, "")
		chk(err)
		if flags.json {
			printmodeljson(name, model)
		} else {
			printmodel(name, model)
		}
	}
}

func printmodel(name string, model apoco.Model) {
	if modelFlags.histPats {
		printpats(name, "hist", model.GlobalHistPatterns)
	}
	if modelFlags.ocrPats {
		printpats(name, "ocr", model.GlobalOCRPatterns)
	}
	if !modelFlags.noWeights {
		for typ, data := range model.Models {
			printmodeldata(name, typ, data)
		}
	}
}

func printmodeldata(name, typ string, ds map[int]apoco.ModelData) {
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

func printmodeljson(name string, model apoco.Model) {
	st := modelst{Name: name}
	if modelFlags.histPats {
		st.GlobalHistPatterns = model.GlobalHistPatterns
	}
	if modelFlags.ocrPats {
		st.GlobalOCRPatterns = model.GlobalOCRPatterns
	}
	if !modelFlags.noWeights {
		st.Features = make(map[string][]feature)
		for typ, data := range model.Models {
			st.Features[typ] = jsonfeatures(typ, data)
		}
	}
	chk(json.NewEncoder(os.Stdout).Encode(st))
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
	Features           map[string][]feature `json:",omitempty"`
	GlobalHistPatterns map[string]float64   `json:",omitempty"`
	GlobalOCRPatterns  map[string]float64   `json:",omitempty"`
}

type feature struct {
	Name   string
	Weight float64
	Nocr   int
}
