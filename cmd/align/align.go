package align

import (
	"log"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

// func init() {
// 	CMD.Flags().IntVarP(&flags.nocr, "nocr", "n", 2, "set nocr")
// }

// var flags = struct {
// 	mets, inputFileGrp, outputFileGrp string
// 	parameters                        string
// 	nocr                              int
// }{}

// CMD defines the apoco align command.
var CMD = &cobra.Command{
	Use:   "align",
	Short: "Align input lines word-wise",
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	words := align(args...)
	for _, x := range words {
		log.Printf("%s", strings.Join(x, "/"))
	}
}

func align(strs ...string) [][]string {
	if len(strs) == 0 {
		return nil
	}
	for i := range strs {
		strs[i] = strings.Trim(strs[i], "\t \n\r\v")
	}
	var spaces []int
	var words [][]string
	b := -1
	for i, r := range strs[0] {
		if unicode.IsSpace(r) {
			spaces = append(spaces, i)
			words = append(words, []string{strs[0][b+1 : i]})
			b = i
		}
	}
	words = append(words, []string{strs[0][b+1:]})
	for i := 1; i < len(strs); i++ {
		alignments := alignAt(spaces, strs[i])
		for j := range words {
			words[j] = append(words[j], alignments[j])
		}
	}
	return words
}

func alignAt(spaces []int, str string) []string {
	ret := make([]string, 0, len(spaces)+1)
	b := -1
	for _, pos := range spaces {
		e := alignmentPos(str, pos)
		ret = append(ret, str[b+1:e])
		if e != len(str) {
			b = e
		}
	}
	ret = append(ret, str[b+1:])
	return ret
}

func alignmentPos(str string, pos int) int {
	if pos >= len(str) {
		return len(str)
	}
	if str[pos] == ' ' {
		return pos
	}
	for i := 1; ; i++ {
		if pos+i >= len(str) && i >= pos {
			return len(str)
		}
		if pos+i < len(str) && str[pos+i] == ' ' {
			return pos + i
		}
		if i <= pos && str[pos-i] == ' ' {
			return pos - i
		}
	}
}
