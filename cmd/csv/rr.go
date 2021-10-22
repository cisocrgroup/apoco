package csv

import (
	"context"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/finkf/gofiler"
	"github.com/spf13/cobra"
)

// rrCMD defines the apoco csv rr command.
var rrCMD = &cobra.Command{
	Use:   "rr [[DIR...] | [FILE...]]",
	Short: "Extract re-ranking features to csv",
	Run:   rrRun,
}

func rrRun(_ *cobra.Command, args []string) {
	c, err := internal.ReadConfig(flags.parameter)
	chk(err)

	internal.UpdateInConfig(&c.Model, flags.model)
	internal.UpdateInConfig(&c.Nocr, flags.nocr)
	internal.UpdateInConfig(&c.Cache, flags.cache)
	internal.UpdateInConfig(&c.AlignLev, flags.alev)

	m, err := internal.ReadModel(c.Model, c.LM, true)
	chk(err)

	p := internal.Piper{
		Exts:     flags.extensions,
		Dirs:     args,
		AlignLev: c.AlignLev,
	}
	chk(p.Pipe(
		context.Background(),
		apoco.FilterBad(c.Nocr+1), // at least n ocr + ground truth
		apoco.Normalize(),
		apoco.FilterShort(4),
		apoco.ConnectLanguageModel(m.LM),
		apoco.ConnectUnigrams(),
		internal.ConnectProfile(c, "-profile.json.gz"),
		apoco.FilterLexiconEntries(),
		apoco.ConnectCandidates(),
		csv(c.RR.Features, c.Nocr, rrGT),
	))
	chk(m.Write(c.Model))
}

func rrGT(t apoco.T) (float64, bool) {
	candidate := t.Payload.(*gofiler.Candidate)
	return ml.Bool(candidate.Suggestion == t.Tokens[len(t.Tokens)-1]), true
}
