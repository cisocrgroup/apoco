package print

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/cobra"
)

// CMD defines the apoco print command.
var CMD = &cobra.Command{
	Use:   "print",
	Short: "Print out information",
}

var flags = struct {
	json bool
}{}

func init() {
	CMD.PersistentFlags().BoolVarP(&flags.json, "json", "J", false, "set json output")
	// Subcommands
	CMD.AddCommand(statsCMD, tokensCMD, modelCMD, protocolCMD, profileCMD, charsetCMD)
}

func parseDTD(dtd string, skip, short, lex, cor *bool, rank *int, ocr, sug, gt *string) error {
	_, err := fmt.Sscanf(dtd, dtdFormat, skip, short, lex, cor, rank, ocr, sug, gt)
	if err != nil {
		return fmt.Errorf("parseDTD: cannot parse %q: %v", dtd, err)
	}
	return nil
}

func e(str string) string {
	if str == "" {
		return "ε"
	}
	return strings.ToLower(strings.Replace(str, " ", "_", -1))
}

func chk(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
