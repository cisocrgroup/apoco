package csv

import (
	"context"
	"fmt"
	"strings"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/spf13/cobra"
)

// ffCMD defines the apoco train ff command.
var ffCMD = &cobra.Command{
	Use:   "ff [DIRS...]",
	Short: "Train an apoco false friends detection model",
	Run:   ffRun,
}

func ffRun(_ *cobra.Command, args []string) {
	c, err := internal.ReadConfig(flags.parameter)
	chk(err)

	internal.UpdateInConfig(&c.Model, flags.model)
	internal.UpdateInConfig(&c.Nocr, flags.nocr)
	internal.UpdateInConfig(&c.Cache, flags.cache)
	internal.UpdateInConfig(&c.AlignLev, flags.alev)

	profile_path := strings.Replace(args[0], "corpus", "profiles-c", -1)
	//	profile_path :=  strings.Replace(args[0],"corpus","profiles-b",-1)

	if strings.Contains(profile_path, "grenzboten") {
		profile_path += "-even"
	}

	if strings.Contains(profile_path, "bodenstein") {
		profile_path += "-even"
	}

	profile_path += ".json.gz"

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
		connectProfileFF(c, profile_path),
		apoco.FilterNonLexiconEntries(),
		fflog(),
		csv(c.FF.Features, c.Nocr, ffGT),
	))
}

func fflog() apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {

		cnt_true := 0
		cnt_false := 0

		err := apoco.EachToken(ctx, in, func(t apoco.T) error {
			if x, _ := ffGT(t); x > 0 {
				fmt.Printf("%s:%f %s\n", t.Tokens[0], x, t.Tokens[len(t.Tokens)-1])
				cnt_true++
			} else {
				cnt_false++
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("log ff: %v", err)
		}
		fmt.Printf("true %d, false %d\n", cnt_true, cnt_false)
		return nil
	}
}

func connectProfileFF(c *internal.Config, profile string) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {

		return apoco.EachDocument(ctx, in, func(d *apoco.Document, tokens []apoco.T) error {
			profile, err := apoco.ReadProfile("profiles-c/" + d.Group + ".json.gz")

			if err != nil {
				return err
			}

			d.Profile = profile
			d.OCRPats = profile.GlobalOCRPatterns()
			return apoco.SendTokens(ctx, out, tokens...)
		})

	}
}

func loadFFModel(c *internal.Config, m *internal.Model, update bool) (*ml.LR, apoco.FeatureSet, error) {
	if update {
		return m.Get("ff", c.Nocr)
	}
	fs, err := apoco.NewFeatureSet(c.FF.Features...)
	if err != nil {
		return nil, nil, err
	}
	lr := &ml.LR{
		LearningRate: c.FF.LearningRate,
		Ntrain:       c.FF.Ntrain,
	}
	return lr, fs, nil
}

func ffGT(t apoco.T) (float64, bool) {
	return ml.Bool(t.Tokens[0] != t.GT()), true
}
