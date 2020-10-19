package printmodel

import (
	"fmt"
	"log"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/finkf/gofiler"
	"github.com/spf13/cobra"
)

// CMD runs the apoco printmodel command.
var CMD = &cobra.Command{
	Use:   "printmodel [MODEL...]",
	Short: "Print information about a model",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
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
		// _, err := fmt.Printf("%s(%d) = %v\n", typ, nocr, data.Features)
		// chk(err)
		// _, err = fmt.Printf("%s(%d) = %v\n", typ, nocr, ws)
		// chk(err)
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

func mktok(typ string, nocr int) apoco.Token {
	switch typ {
	case "dm":
		return apoco.Token{
			Tokens: make([]string, nocr),
			Payload: []apoco.Ranking{
				apoco.Ranking{Candidate: new(gofiler.Candidate)},
			},
		}
	default:
		return apoco.Token{
			Tokens:  make([]string, nocr),
			Payload: new(gofiler.Candidate),
		}
	}
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
