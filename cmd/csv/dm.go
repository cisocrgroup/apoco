package csv

import (
	"context"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/spf13/cobra"
)

// dmCMD defines the apoco train command.
var dmCMD = &cobra.Command{
	Use:   "dm [[DIR...] | [FILE...]]",
	Short: "Extract decision maker features to csv",
	Run:   dmRun,
}

var dmFlags = struct {
	filter string
}{}

func init() {
	dmCMD.Flags().StringVarP(&dmFlags.filter, "filter", "f", "courageous",
		"set courageous, redundant or cautious training filter")
}

func dmRun(_ *cobra.Command, args []string) {
	c, err := internal.ReadConfig(flags.parameter)
	chk(err)

	internal.UpdateInConfig(&c.Model, flags.model)
	internal.UpdateInConfig(&c.Nocr, flags.nocr)
	internal.UpdateInConfig(&c.Cache, flags.cache)
	internal.UpdateInConfig(&c.AlignLev, flags.alev)
	internal.UpdateInConfig(&c.Lex, flags.lex)
	internal.UpdateInConfig(&c.DM.Filter, dmFlags.filter)

	m, err := internal.ReadModel(c.Model, c.LM, true)
	chk(err)
	lr, fs, err := m.Get("rr", c.Nocr)
	chk(err)
	p := internal.Piper{
		Exts: flags.extensions,
		Dirs: args,
	}
	chk(p.Pipe(
		context.Background(),
		apoco.FilterBad(c.Nocr+1), // at least n ocr + ground truth
		apoco.Normalize(),
		apoco.FilterShort(4),
		apoco.ConnectLanguageModel(m.LM),
		apoco.ConnectUnigrams(),
		internal.ConnectProfile(c, "-profile.json.gz"),
		internal.FilterLex(c),
		apoco.ConnectCandidates(),
		apoco.ConnectRankings(lr, fs, c.Nocr),
		csv(c.DM.Features, c.Nocr, dmGT(dmFlags.filter)),
	))
	chk(m.Write(c.Model))
}

func useTokenForDMTraining(t apoco.T, filter string) bool {
	if filter == internal.Cautious {
		return true
	}
	ocr := t.Tokens[0]
	gt := t.Tokens[len(t.Tokens)-1]
	// If ocr != gt we use the token if the correction suggestion is correct.
	// We skip token with "don't care corrections" (incorrect correction
	// for an incorrect ocr token).
	if ocr != gt {
		return t.Payload.([]apoco.Ranking)[0].Candidate.Suggestion == gt
	}
	// We do not want to train with redundant corrections (ocr == gt && sugg == gt).
	// If ocr == gt and sugg == gt we skip the token for the training.
	// Note that at this point ocr == gt holds (see above).
	if filter == internal.Redundant {
		return t.Payload.([]apoco.Ranking)[0].Candidate.Suggestion != gt
	}
	return true
}

func dmGT(filter string) func(apoco.T) (float64, bool) {
	return func(t apoco.T) (float64, bool) {
		use := useTokenForDMTraining(t, filter)
		sug := t.Payload.([]apoco.Ranking)[0].Candidate.Suggestion
		gt := t.Tokens[len(t.Tokens)-1]
		return ml.Bool(sug == gt), use
	}
}
