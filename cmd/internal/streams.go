package internal

import "git.sr.ht/~flobar/apoco/pkg/apoco"

func FilterLex(c *Config) apoco.StreamFunc {
	if c.Lex {
		return apoco.FilterNonLexiconEntries()
	}
	return apoco.FilterLexiconEntries()
}
