package eval

import (
	"context"
	"fmt"
	"os"
	"strings"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/spf13/cobra"
	"gonum.org/v1/gonum/mat"
)

// ffCMD defines the apoco train ff command.
var ffCMD = &cobra.Command{
	Use:   "ff [DIRS...]",
	Short: "Evaluate an apoco false-friends model",
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
		connectProfileFF_single(c, profile_path),
		apoco.FilterNonLexiconEntries(),
		ffEval(c, m),
	))
}

func ffEval(c *internal.Config, m *internal.Model) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		lr, fs, err := m.Get("ff", c.Nocr)
		if err != nil {
			return fmt.Errorf("ffeval: %v", err)
		}
		var ts []apoco.T

		var xs, ys []float64
		err = apoco.EachToken(ctx, in, func(t apoco.T) error {
			xs = fs.Calculate(xs, t, c.Nocr)
			ys = append(ys, ffGT(t))
			ts = append(ts, t)
			return nil
		})
		if err != nil {
			return fmt.Errorf("ffeval: %v", err)
		}
		var s stats
		ps := lr.Predict(mat.NewDense(len(ys), len(xs)/len(ys), xs))
		// s.eval(lr, 0.5, xs, ys)
		// s.print(os.Stdout, "dm", c.Nocr)

		for i := 0; i < len(ys); i++ {
			//cs := ts[i].Document.Profile[ts[i].Tokens[0]]
			//fmt.Printf("Candidates: %d\n",len(cs.Candidates))

			switch s.add(ys[i], ps.AtVec(i)) {
			case tp:
				//fmt.Printf("True Positive: " + ts[i].Tokens[0]+ "  :  "+ts[i].GT()+"\n")
			case fp:
				//fmt.Printf("False Positive: " + ts[i].Tokens[0]+"\n")
			}

		}
		return s.print(os.Stdout, "ff", c.Nocr)
	}
}

func connectProfileFF_single(c *internal.Config, profile string) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		profile, err := apoco.ReadProfile(profile)
		if err != nil {
			return err
		}
		return apoco.ConnectProfile(profile)(ctx, in, out)
	}
}

func ffGT(t apoco.T) float64 {
	return ml.Bool(t.Tokens[0] != t.GT())
}
