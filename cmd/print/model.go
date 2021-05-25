package print

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
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
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	defer func() {
		chk(w.Flush())
	}()
	if modelFlags.histPats {
		printpats(w, name, "hist", model.GlobalHistPatterns)
	}
	if modelFlags.ocrPats {
		printpats(w, name, "ocr", model.GlobalOCRPatterns)
	}
	if !modelFlags.noWeights {
		for _, typ := range []string{"rr", "dm"} {
			printmodeldata(w, name, typ, model.Models[typ])
		}
	}
}

func printmodeldata(out io.Writer, name, typ string, ds map[int]apoco.ModelData) {
	for nocr, data := range ds {
		ws := data.Model.Weights()
		fs, err := apoco.NewFeatureSet(data.Features...)
		chk(err)
		names := fs.Names(data.Features, nocr, typ == "dm")
		if len(names) != len(ws) {
			panic("bad feature names")
		}
		for i := range names {
			_, err := fmt.Fprintf(out, "%s\t%s/%d\t%s\t%g\n",
				name, typ, nocr, names[i], ws[i])
			chk(err)
		}
	}
}

func printpats(out io.Writer, name, typ string, pats map[string]float64) {
	for pat, prob := range pats {
		_, err := fmt.Fprintf(out, "%s\t%s\t%s\t%g\n", name, typ, pat, prob)
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
		names := fs.Names(data.Features, nocr, typ == "dm")
		if len(names) != len(ws) {
			panic("bad feature names")
		}
		for i := range names {
			features = append(features, feature{
				Name:      names[i],
				Nocr:      nocr,
				Weight:    ws[i],
				Error:     data.Model.Error(),
				Instances: data.Model.Instances(),
			})
		}
	}
	return features
}

type modelst struct {
	Name               string
	Features           map[string][]feature `json:",omitempty"`
	GlobalHistPatterns map[string]float64   `json:",omitempty"`
	GlobalOCRPatterns  map[string]float64   `json:",omitempty"`
}

type feature struct {
	Name      string
	Weight    float64
	Nocr      int
	Error     float64
	Instances int
}
