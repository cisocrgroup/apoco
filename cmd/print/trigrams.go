package print

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/spf13/cobra"
)

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
	if flags.json {
		chk(json.NewEncoder(os.Stdout).Encode(trigrams))
		return
	}
	for k, v := range trigrams {
		_, err := fmt.Printf("%d,%s\n", v, k)
		chk(err)
	}
}
