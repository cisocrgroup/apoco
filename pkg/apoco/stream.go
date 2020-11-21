package apoco

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"git.sr.ht/~flobar/apoco/pkg/apoco/ml"
	"github.com/finkf/gofiler"
	"golang.org/x/sync/errgroup"
	"gonum.org/v1/gonum/mat"
)

// StreamFunc is a type def for stream funcs.
type StreamFunc func(context.Context, *errgroup.Group, <-chan Token) (<-chan Token, error)

// type StreamFunc func(context.Context, chan<- Token, <-chan Token) error

// Pipe pipes multiple stream funcs together, making shure to run all
// of them in paralell.  The first function in the list (the reader)
// is called with a nil channel.  It is required for the last stream
// function to always return a nil channel; otherwise this function
// panics.
//
// TODO: Pipe should not return a channel.
// TODO: Pipe should explicitly use g.Go(func(){...}) so that the stream
//       functions don't need to use g.Go themselves.
func Pipe(ctx context.Context, g *errgroup.Group, r StreamFunc, ps ...StreamFunc) {
	out := r(ctx, g, nil)
	for _, p := range ps {
		out = p(ctx, g, out)
	}
	if (out) != nil {
		panic("pipe: last function did not return a nil channel")
	}
	return out
}

// EachToken iterates over the tokens in the input channel and calls
// the callback function for each token.
func EachToken(ctx context.Context, in <-chan Token, f func(Token) error) error {
	for {
		token, ok, err := ReadToken(ctx, in)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err := f(token); err != nil {
			return err
		}
	}
}

// EachTokenGroup iterates over the tokens grouping them together
// based on their groups.  The given callback function is called for
// each group of tokens.
func EachTokenGroup(ctx context.Context, in <-chan Token, f func(string, ...Token) error) error {
	var group string
	var tokens []Token
	err := EachToken(ctx, in, func(t Token) error {
		if group == "" {
			group = t.Group
		}
		if group != t.Group {
			if err := f(group, tokens...); err != nil {
				return fmt.Errorf("eachTokenGroup: %v", err)
			}
			tokens = tokens[0:0] // Clear token array.
			group = t.Group
			return nil
		}
		tokens = append(tokens, t)
		return nil
	})
	// Handle last group of tokens.
	if len(tokens) != 0 {
		if err := f(group, tokens...); err != nil {
			return fmt.Errorf("eachTokenGroup: %v", err)
		}
	}
	if err != nil {
		return fmt.Errorf("eachTokenGroup: %v", err)
	}
	return nil
}

// ReadToken reads one token from the given channel.
func ReadToken(ctx context.Context, in <-chan Token) (Token, bool, error) {
	select {
	case token, ok := <-in:
		if !ok {
			return token, false, nil
		}
		return token, true, nil
	case <-ctx.Done():
		return Token{}, false, fmt.Errorf("readToken: %v", ctx.Err())
	}
}

// SendTokens writes tokens into the given output channel.
func SendTokens(ctx context.Context, out chan<- Token, tokens ...Token) error {
	for _, t := range tokens {
		select {
		case out <- t:
		case <-ctx.Done():
			return fmt.Errorf("sendToken: %v", ctx.Err())
		}
	}
	return nil
}

// Normalize trims all leading and subsequent punctionation from the
// tokens, converts them to lowercase and replaces any whitespace
// (in the case of merges due to alignment) with a '_'.
func Normalize(ctx context.Context, g *errgroup.Group, in <-chan Token) <-chan Token {
	out := make(chan Token)
	g.Go(func() error {
		defer close(out)
		return EachToken(ctx, in, func(t Token) error {
			for i := range t.Tokens {
				if i == 0 { // handle master OCR in a special way
					t.Chars = normalizeChars(t.Chars)
				}
				t.Tokens[i] = strings.TrimFunc(t.Tokens[i], func(r rune) bool {
					return unicode.IsPunct(r) || unicode.IsSpace(r)
				})
				t.Tokens[i] = strings.ReplaceAll(strings.ToLower(t.Tokens[i]), " ", "_")
			}
			if err := SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("normalize: %v", err)
			}
			return nil
		})
	})
	return out
}

func charsToString(chars Chars) string {
	var b strings.Builder
	for _, char := range chars {
		b.WriteRune(char.Char)
	}
	return b.String()
}

func normalizeChars(chars Chars) Chars {
	var i, j int
	for i = 0; i < len(chars); i++ {
		if !(unicode.IsPunct(chars[i].Char) || unicode.IsSpace(chars[i].Char)) {
			break
		}
	}
	for j = len(chars); j > i; j-- {
		if !(unicode.IsPunct(chars[j-1].Char) || unicode.IsSpace(chars[i].Char)) {
			break
		}
	}
	return chars[i:j]
}

// FilterBad filters tokens with not enough ocr and/or gt tokens.
func FilterBad(min int) StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan Token) <-chan Token {
		out := make(chan Token)
		g.Go(func() error {
			defer close(out)
			err := EachToken(ctx, in, func(t Token) error {
				if len(t.Tokens) < min {
					return nil
				}
				if err := SendTokens(ctx, out, t); err != nil {
					return fmt.Errorf("filterBad: %v", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("filterBad: %v", err)
			}
			return nil
		})
		return out

	}
}

// FilterShort filters short master OCR tokens from the input stream.
// Short tokens are tokens, with less than 4 unicode characters.
func FilterShort(ctx context.Context, g *errgroup.Group, in <-chan Token) <-chan Token {
	out := make(chan Token)
	g.Go(func() error {
		defer close(out)
		err := EachToken(ctx, in, func(t Token) error {
			if utf8.RuneCountInString(t.Tokens[0]) <= 3 {
				return nil
			}
			if err := SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("filterShort: %v", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("filterShort: %v", err)
		}
		return nil
	})
	return out
}

// FilterLexiconEntries filters all tokens that are lexicon entries
// from the stream.
func FilterLexiconEntries(ctx context.Context, g *errgroup.Group, in <-chan Token) <-chan Token {
	out := make(chan Token)
	g.Go(func() error {
		defer close(out)
		err := EachToken(ctx, in, func(t Token) error {
			if t.IsLexiconEntry() {
				return nil
			}
			if err := SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("filterLexiconEntry: %v", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("filterLexiconEntry: %v", err)
		}
		return nil
	})
	return out
}

// ConnectCandidates connects tokens with their respective candidates
// to the stream.  Tokens with no candidates or tokens with only a
// modern interpretation are filtered from the stream.
func ConnectCandidates(ctx context.Context, g *errgroup.Group, in <-chan Token) <-chan Token {
	out := make(chan Token)
	g.Go(func() error {
		defer close(out)
		err := EachToken(ctx, in, func(t Token) error {
			interp, ok := t.LM.Profile[t.Tokens[0]]
			if !ok { // no suggestions (too short or unknown)
				return nil
			}
			for i := range interp.Candidates {
				t.Payload = &interp.Candidates[i]
				if err := SendTokens(ctx, out, t); err != nil {
					return fmt.Errorf("add candidate: %v", err)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("add candidate: %v", err)
		}
		return nil
	})
	return out
}

// ConnectLM loads the language model for the tokens and adds them to
// each token.  Based on the file group of the tokens different
// language models are loaded.
func ConnectLM(c *Config, ngrams FreqList) StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan Token) <-chan Token {
		out := make(chan Token)
		g.Go(func() error {
			defer close(out)
			err := EachTokenGroup(ctx, in, func(group string, tokens ...Token) error {
				loader := lmLoader{
					config: c,
					lm:     &LanguageModel{ngrams: ngrams},
					tokens: tokens,
				}
				if err := loader.loadAndSend(ctx, out); err != nil {
					return fmt.Errorf("connectLM: %v", err)
				}
				return nil

			})
			if err != nil {
				return fmt.Errorf("connectLM: %v", err)
			}
			return nil
		})
		return out
	}
}

type lmLoader struct {
	lm     *LanguageModel
	tokens []Token
	config *Config
}

func (l lmLoader) loadAndSend(ctx context.Context, out chan<- Token) error {
	if err := l.load(ctx); err != nil {
		return fmt.Errorf("loadAndSend: %v", err)
	}
	if err := SendTokens(ctx, out, l.tokens...); err != nil {
		return fmt.Errorf("loadAndSend: %v", err)
	}
	return nil
}

func (l lmLoader) load(ctx context.Context) error {
	var g errgroup.Group
	g.Go(func() error {
		err := l.lm.LoadProfile(
			ctx,
			l.config.ProfilerBin,
			l.config.ProfilerConfig,
			l.config.Cache,
			l.tokens...,
		)
		return err
	})
	for i := range l.tokens {
		l.tokens[i].LM = l.lm
		l.lm.AddUnigram(l.tokens[i].Tokens[0])
	}
	return g.Wait()
}

// ConnectRankings connects the tokens of the input stream with their
// respective rankings.
func ConnectRankings(lr *ml.LR, fs FeatureSet, n int) StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan Token) <-chan Token {
		out := make(chan Token)
		g.Go(func() error {
			defer close(out)
			var lfid, ltid string // last file id and last token id
			var tokens []Token
			err := EachToken(ctx, in, func(t Token) error {
				if t.File != lfid || t.ID != ltid {
					if len(tokens) > 0 {
						tmp := connectRankings(lr, fs, n, tokens)
						if err := SendTokens(ctx, out, tmp); err != nil {
							return err
						}
						tokens = tokens[0:0]
					}
					lfid = t.File
					ltid = t.ID
				}
				tokens = append(tokens, t)
				return nil
			})
			if err != nil {
				return err
			}
			if len(tokens) > 0 {
				t := connectRankings(lr, fs, n, tokens)
				if err := SendTokens(ctx, out, t); err != nil {
					return err
				}
			}
			return nil
		})
		return out
	}
}

func connectRankings(lr *ml.LR, fs FeatureSet, n int, tokens []Token) Token {
	var xs []float64
	// calculate feature values
	for _, token := range tokens {
		xs = fs.Calculate(xs, token, n)
	}
	// calculate prediction probabilities
	xmat := mat.NewDense(len(tokens), len(xs)/len(tokens), xs)
	probs := lr.PredictProb(xmat)
	rankings := make([]Ranking, len(tokens))
	// probs, tokens and rankings all have the same length
	for i := range tokens {
		rankings[i].Prob = probs.AtVec(i)
		rankings[i].Candidate = tokens[i].Payload.(*gofiler.Candidate)
	}
	// sort from highest probability to lowest probability
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[j].Prob < rankings[i].Prob
	})
	tokens[0].Payload = rankings
	return tokens[0]
}

// ConnectCorrections connects the tokens with the decider's correction
// decisions.
func ConnectCorrections(lr *ml.LR, fs FeatureSet, n int) StreamFunc {
	return func(ctx context.Context, g *errgroup.Group, in <-chan Token) <-chan Token {
		out := make(chan Token)
		g.Go(func() error {
			defer close(out)
			blen := 1024
			buf := make([]Token, 0, blen)
			err := EachToken(ctx, in, func(t Token) error {
				if len(buf) >= blen {
					connectCorrections(lr, fs, n, buf)
					if err := SendTokens(ctx, out, buf...); err != nil {
						return fmt.Errorf("connectCorrections: %v", err)
					}
					buf = buf[0:0]
				}
				buf = append(buf, t)
				return nil
			})
			if err != nil {
				return fmt.Errorf("connectCorrections: %v", err)
			}
			if len(buf) > 0 {
				connectCorrections(lr, fs, n, buf)
				if err := SendTokens(ctx, out, buf...); err != nil {
					return fmt.Errorf("connectCorrections: %v", err)
				}
			}
			return nil
		})
		return out
	}
}

func connectCorrections(lr *ml.LR, fs FeatureSet, nocr int, tokens []Token) {
	xs := make([]float64, 0, len(tokens)*len(fs))
	for _, t := range tokens {
		xs = fs.Calculate(xs, t, nocr)
	}
	x := mat.NewDense(len(tokens), len(xs)/len(tokens), xs)
	p := lr.PredictProb(x)
	for i := range tokens {
		tokens[i].Payload = Correction{
			Candidate: tokens[i].Payload.([]Ranking)[0].Candidate,
			Conf:      p.AtVec(i),
		}
	}
}
