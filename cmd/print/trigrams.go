package print

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/spf13/cobra"
)

var trigramsFlags = struct {
	clean int
}{}

func init() {
	trigramsCMD.Flags().IntVarP(&trigramsFlags.clean, "clean", "c",
		0, "remove trigrams with less than arg occurences")
}

// trigramsCMD runs the apoco print trigrams subcommand.
var trigramsCMD = &cobra.Command{
	Run:   runTrigrams,
	Use:   "trigrams",
	Short: "Generate language model trigrams",
	Long: `
Generate language model trigrams from a line separated token list.
The tokens are read from stdin and written to stdout.  Each token
should be on it's own line.`,
}

func runTrigrams(_ *cobra.Command, args []string) {
	trigrams := make(map[string]int)
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		apoco.EachTrigram(s.Text(), func(trigram string) {
			trigrams[trigram] += 1
		})
	}
	chk(s.Err())
	if trigramsFlags.clean > 0 {
		clean(trigrams, trigramsFlags.clean)
	}
	if flags.json {
		chk(json.NewEncoder(os.Stdout).Encode(trigrams))
		return
	}
	if flags.json {
		chk(json.NewEncoder(os.Stdout).Encode(trigrams))
		return
	}
	for k, v := range trigrams {
		_, err := fmt.Printf("%d,%s\n", v, k)
		chk(err)
	}
}

func clean(trigrams map[string]int, t int) {
	for k, v := range trigrams {
		if v <= t {
			delete(trigrams, k)
		}
	}
}
