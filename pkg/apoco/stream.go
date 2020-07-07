package apoco

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"example.com/apoco/pkg/apoco/ml"
	"github.com/finkf/gofiler"
	"golang.org/x/sync/errgroup"
	"gonum.org/v1/gonum/mat"
)

// StreamFunc is a type def for stream funcs.
type StreamFunc func(context.Context, *errgroup.Group, <-chan Token) <-chan Token

// Pipe pipes multiple stream funcs together.  The first function in the list (the reader)
// is called with a nil channel
func Pipe(ctx context.Context, g *errgroup.Group, r StreamFunc, ps ...StreamFunc) <-chan Token {
	out := r(ctx, g, nil)
	for _, p := range ps {
		out = p(ctx, g, out)
	}
	return out
}

// EachToken iterates over the tokens in the input channel and calls
// the callback function for each token.
func EachToken(ctx context.Context, in <-chan Token, f func(Token) error) error {
	for {
		token, ok, err := ReadToken(ctx, in)
		if err != nil {
			return fmt.Errorf("eachToken: %v", err)
		}
		if !ok {
			return nil
		}
		if err := f(token); err != nil {
			return fmt.Errorf("eachToken: %v", err)
		}
	}
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
// tokens and converts them to lowercase.
func Normalize(ctx context.Context, g *errgroup.Group, in <-chan Token) <-chan Token {
	out := make(chan Token)
	g.Go(func() error {
		defer close(out)
		return EachToken(ctx, in, func(t Token) error {
			for i := range t.Tokens {
				t.Tokens[i] = strings.TrimFunc(t.Tokens[i], func(r rune) bool {
					return unicode.IsPunct(r)
				})
				t.Tokens[i] = strings.ToLower(t.Tokens[i])
			}
			if err := SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("normalize: %v", err)
			}
			return nil
		})
	})
	return out
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
			var fg string
			loader := lmLoader{config: c, lm: &LanguageModel{ngrams: ngrams}}
			err := EachToken(ctx, in, func(t Token) error {
				if fg == "" {
					fg = t.FileGroup
				}
				if fg != t.FileGroup { // new file group
					if err := loader.load(ctx); err != nil {
						return fmt.Errorf("connectLM: %v", err)
					}
					if err := SendTokens(ctx, out, loader.tokens...); err != nil {
						return fmt.Errorf("connectLM: %v", err)
					}
					loader.tokens = loader.tokens[:]
					fg = t.FileGroup
				}
				loader.tokens = append(loader.tokens, t)
				loader.lm = &LanguageModel{ngrams: ngrams}
				return nil
			})
			if err != nil {
				return fmt.Errorf("connectLM: %v", err)
			}
			if len(loader.tokens) > 0 {
				if err := loader.load(ctx); err != nil {
					return fmt.Errorf("connectLM: %v", err)
				}
				if err := SendTokens(ctx, out, loader.tokens...); err != nil {
					return fmt.Errorf("connectLM: %v", err)
				}
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

func (l lmLoader) load(ctx context.Context) error {
	var g errgroup.Group
	g.Go(func() error {
		err := l.lm.LoadProfile(
			ctx,
			l.config.ProfilerBin,
			l.config.ProfilerConfig,
			l.config.NoCache,
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
		vals := fs.Calculate(token, n)
		xs = append(xs, vals...)
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
		vals := fs.Calculate(t, nocr)
		xs = append(xs, vals...)
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
