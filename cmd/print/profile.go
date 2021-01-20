package print

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"

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
		profile, err := readProfile(name)
		chk(err)
		if flags.json {
			printprofilejson(name, profile)
		} else {
			printprofile(name, profile)
		}
	}
}

func printprofile(name string, profile gofiler.Profile) {
	if profileFlags.histPats {
		printpats(name, "hist", profile.GlobalHistPatterns())
	}
	if profileFlags.ocrPats {
		printpats(name, "ocr", profile.GlobalOCRPatterns())
	}
	if !profileFlags.noProfile {
		for _, i := range profile {
			for j, c := range i.Candidates {
				fmt.Printf("%s %d %s %s\n", name, j+1, i.OCR, c)
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

func readProfile(name string) (gofiler.Profile, error) {
	in, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("read profile %s: %v", name, err)
	}
	defer in.Close()
	r, err := gzip.NewReader(in)
	if err != nil {
		return nil, fmt.Errorf("read profile %s: %v", name, err)
	}
	var profile gofiler.Profile
	if err := json.NewDecoder(r).Decode(&profile); err != nil {
		return nil, fmt.Errorf("read profile %s: %v", name, err)
	}
	return profile, nil
}

type profilest struct {
	Name               string
	Profile            gofiler.Profile    `json:",omitempty"`
	GlobalHistPatterns map[string]float64 `json:",omitempty"`
	GlobalOCRPatterns  map[string]float64 `json:",omitempty"`
}
