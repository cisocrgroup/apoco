package profile

import (
	"context"
	"log"

	"git.sr.ht/~flobar/apoco/cmd/internal"
	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/spf13/cobra"
)

// CMD defines the apoco profile command.
var CMD = &cobra.Command{
	Use:   "profile DIR [DIRS...] OUT",
	Short: "Create profiles of documents",
	Args:  cobra.MinimumNArgs(2),
	Run:   runProfile,
}

var flags = struct {
	extensions []string
	parameters string
}{}

func init() {
	CMD.PersistentFlags().StringVarP(&flags.parameters, "parameters", "P", "config.toml",
		"set the path to the configuration file")
	CMD.PersistentFlags().StringSliceVarP(&flags.extensions, "extensions", "e", []string{".xml"},
		"set the input file extensions")
}

func runProfile(_ *cobra.Command, args []string) {
	c, err := apoco.ReadConfig(flags.parameters)
	chk(err)
	p := internal.Piper{
		Exts: flags.extensions,
		Dirs: args[:len(args)-1],
	}
	chk(p.Pipe(
		context.Background(),
		apoco.FilterBad(1),
		apoco.Normalize(),
		apoco.FilterShort(4),
		writeProfile(c, args[len(args)-1]),
	))
}

func writeProfile(c *apoco.Config, name string) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, _ chan<- apoco.T) error {
		var ts []apoco.T
		err := apoco.EachToken(ctx, in, func(t apoco.T) error {
			ts = append(ts, t)
			return nil
		})
		if err != nil {
			return err
		}
		profile, err := apoco.RunProfiler(ctx, c.ProfilerBin, c.ProfilerConfig, ts...)
		if err != nil {
			return err
		}
		return apoco.WriteProfile(name, profile)
	}
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
