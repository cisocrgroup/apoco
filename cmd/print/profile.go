package print

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/finkf/gofiler"
	"github.com/spf13/cobra"
)

// profileCMD runs the apoco print profile command.
var profileCMD = &cobra.Command{
	Use:   "profile [PROFILE...]",
	Short: "Print information about profiles",
	Run:   runProfile,
}

var profileFlags = struct {
	histPats, ocrPats, noProfile bool
}{}

func init() {
	profileCMD.Flags().BoolVarP(&profileFlags.histPats, "histpats", "p", false,
		"output global historical pattern probabilities")
	profileCMD.Flags().BoolVarP(&profileFlags.ocrPats, "ocrpats", "e", false,
		"output global ocr error pattern probabilities")
	profileCMD.Flags().BoolVarP(&profileFlags.noProfile, "noprofile", "n", false,
		"do not output profile interpretations ")
}

func runProfile(_ *cobra.Command, args []string) {
	for _, name := range args {
		profile, err := apoco.ReadProfile(name)
		chk(err)
		if flags.json {
			printprofilejson(name, profile)
		} else {
			printprofile(name, profile)
		}
	}
}

func printprofile(name string, profile gofiler.Profile) {
	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	defer func() {
		chk(w.Flush())
	}()
	if profileFlags.histPats {
		printpats(w, name, "hist", profile.GlobalHistPatterns())
	}
	if profileFlags.ocrPats {
		printpats(w, name, "ocr", profile.GlobalOCRPatterns())
	}
	if !profileFlags.noProfile {
		for _, i := range profile {
			for j, c := range i.Candidates {
				fmt.Fprintf(w, "%s %d %s %s\n", name, j+1, i.OCR, c)
			}
		}
	}
}

func printprofilejson(name string, profile gofiler.Profile) {
	st := profilest{Name: name}
	if profileFlags.histPats {
		st.GlobalHistPatterns = profile.GlobalHistPatterns()
	}
	if profileFlags.ocrPats {
		st.GlobalOCRPatterns = profile.GlobalOCRPatterns()
	}
	if !profileFlags.noProfile {
		st.Profile = profile
	}
	chk(json.NewEncoder(os.Stdout).Encode(st))
}

type profilest struct {
	Name               string
	Profile            gofiler.Profile    `json:",omitempty"`
	GlobalHistPatterns map[string]float64 `json:",omitempty"`
	GlobalOCRPatterns  map[string]float64 `json:",omitempty"`
}
