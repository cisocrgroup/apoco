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

// StreamFunc is a type def for stream functions.  A stream function
// is used to transform tokens from the input channel to the output
// channel.  They should be used with the Pipe function to chain
// multiple functions together.
type StreamFunc func(context.Context, <-chan T, chan<- T) error

// Pipe pipes multiple stream funcs together, making shure to run all
// of them concurently.  The first function in the list (the reader)
// is called with a nil input channel.  The last function is always
// called with a nil output channel.  To clarify: the first function
// must never read from its input channel and the last function must
// never write to its output channel.
//
// StreamFunctions should transform the input tokens to output
// tokens. They must never close any channels.  They should use the
// SendTokens, ReadToken and EachToken utility functions to ensure
// proper handling of context cancelation.
func Pipe(ctx context.Context, fns ...StreamFunc) error {
	if len(fns) == 0 {
		return nil
	}
	g, gctx := errgroup.WithContext(ctx)
	var in chan T
	for i, fn := range fns {
		// The last function gets a nil write channel
		var out chan T
		if i < len(fns)-1 {
			out = make(chan T)
		}
		// Wrap into a function to avoid problems with
		// closures and go routines.
		func(fn StreamFunc, in <-chan T, out chan<- T) {
			g.Go(func() error {
				defer func() {
					if out != nil {
						close(out)
					}
				}()
				return fn(gctx, in, out)
			})
		}(fn, in, out)
		in = out
	}
	return g.Wait()
}

// Combine lets you combine stream functions.  All functions are run
// concurently in their own error group.
func Combine(ctx context.Context, fns ...StreamFunc) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		if len(fns) == 0 {
			return nil
		}
		g, gctx := errgroup.WithContext(ctx)
		run := func(fn StreamFunc, in <-chan T, out chan<- T) {
			g.Go(func() error {
				defer close(out)
				return fn(gctx, in, out)
			})
		}
		n := len(fns) // len(fns) != 0
		for _, fn := range fns[:n-1] {
			xout := make(chan T)
			run(fn, in, xout)
			in = xout
		}
		// We do not need to close the last out channel, since
		// this will be closed by the pipe function.
		g.Go(func() error { return fns[n-1](gctx, in, out) })
		return g.Wait()
	}
}

// Tee calls all the given callback function for each token.  After
// all functions have been called, if the output channel is not nil,
// the token is send to the output channel.
func Tee(fns ...func(T) error) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		return EachToken(ctx, in, func(t T) error {
			for _, fn := range fns {
				if err := fn(t); err != nil {
					return fmt.Errorf("tee: %v", err)
				}
			}
			if out != nil {
				if err := SendTokens(ctx, out, t); err != nil {
					return fmt.Errorf("tee: %v", err)
				}
			}
			return nil
		})
	}
}

// EachToken iterates over the tokens in the input channel and calls
// the callback function for each token.
func EachToken(ctx context.Context, in <-chan T, f func(T) error) error {
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
func EachTokenGroup(ctx context.Context, in <-chan T, f func(string, ...T) error) error {
	var group string
	var tokens []T
	err := EachToken(ctx, in, func(t T) error {
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

// EachTokenLM iterates over the tokens grouping them together based on
// their language models. The given callback function is called for
// each group of tokens.  This function must assumes that the tokens are
// connected with a language model.
func EachTokenLM(ctx context.Context, in <-chan T, f func(*LanguageModel, ...T) error) error {
	var lm *LanguageModel
	var tokens []T
	err := EachToken(ctx, in, func(t T) error {
		if lm == nil {
			lm = t.LM
		}
		if lm != t.LM {
			if err := f(lm, tokens...); err != nil {
				return fmt.Errorf("each token language model: %v", err)
			}
			tokens = tokens[0:0] // Clear token array.
			lm = t.LM
			return nil
		}
		tokens = append(tokens, t)
		return nil
	})
	// Handle last group of tokens.
	if len(tokens) != 0 {
		if err := f(lm, tokens...); err != nil {
			return fmt.Errorf("each token language model: %v", err)
		}
	}
	if err != nil {
		return fmt.Errorf("each token language model: %v", err)
	}
	return nil
}

// ReadToken reads one token from the given channel.  This function
// should alsways be used to read single tokens from input channels.
func ReadToken(ctx context.Context, in <-chan T) (T, bool, error) {
	select {
	case token, ok := <-in:
		if !ok {
			return token, false, nil
		}
		return token, true, nil
	case <-ctx.Done():
		return T{}, false, fmt.Errorf("readToken: %v", ctx.Err())
	}
}

// SendTokens writes tokens into the given output channel.  This
// function should always be used to write tokens into output
// channels.
func SendTokens(ctx context.Context, out chan<- T, tokens ...T) error {
	for _, t := range tokens {
		select {
		case out <- t:
		case <-ctx.Done():
			return fmt.Errorf("sendTokens: %v", ctx.Err())
		}
	}
	return nil
}

// Normalize returns a stream function that trims all leading and
// subsequent punctionation from the tokens, converts them to
// lowercase and replaces any whitespace (in the case of merges due to
// alignment) with a '_'.
func Normalize() StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		err := EachToken(ctx, in, func(t T) error {
			for i := range t.Tokens {
				if i == 0 { // handle master OCR in a special way
					t.Chars = normalizeChars(t.Chars)
				}
				t.Tokens[i] = strings.TrimFunc(t.Tokens[i], func(r rune) bool {
					return unicode.IsPunct(r) || unicode.IsSpace(r)
				})
				t.Tokens[i] = strings.ReplaceAll(
					strings.ToLower(t.Tokens[i]), " ", "_")
			}
			if err := SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("normalize: %v", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("normalize: %v", err)
		}
		return nil
	}
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

// FilterBad returns a astream function that filters tokens with not
// enough ocr and/or gt tokens.
func FilterBad(min int) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		err := EachToken(ctx, in, func(t T) error {
			if len(t.Tokens) < min {
				return nil
			}
			if err := SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("filter bad: send tokens: %v", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("filter bad: each token: %v", err)
		}
		return nil
	}
}

// FilterShort returns a stream function that filters short master OCR
// tokens from the input stream.  Short tokens are tokens, with less
// than min unicode characters.
func FilterShort(min int) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		err := EachToken(ctx, in, func(t T) error {
			if utf8.RuneCountInString(t.Tokens[0]) < min {
				return nil
			}
			if err := SendTokens(ctx, out, t); err != nil {
				return fmt.Errorf("filter short: send tokens: %v", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("filter short: each token: %v", err)
		}
		return nil
	}
}

// FilterLexiconEntries returns a stream function that filters all
// tokens that are lexicon entries from the stream.
func FilterLexiconEntries() StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		err := EachToken(ctx, in, func(t T) error {
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
	}
}

// ConnectCandidates returns a stream function that connects tokens
// with their respective candidates to the stream.  Tokens with no
// candidates or tokens with only a modern interpretation are filtered
// from the stream.
func ConnectCandidates() StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		err := EachToken(ctx, in, func(t T) error {
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
	}
}

// ConnectProfiler connects the profile with the tokens.  This function
// must be called after ConnectLM.
func ConnectProfile(exe, config string, cache bool) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		err := EachTokenLM(ctx, in, func(lm *LanguageModel, tokens ...T) error {
			if err := lm.LoadProfile(ctx, exe, config, cache, tokens...); err != nil {
				return fmt.Errorf("connect profile %s %s: %v", exe, config, err)
			}
			if err := SendTokens(ctx, out, tokens...); err != nil {
				return fmt.Errorf("connect profile %s %s: %v", exe, config, err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("connect profile %s %s: %v", exe, config, err)
		}
		return nil
	}
}

// ConnectUnigrams adds the unigrams to the tokens's language model.
func ConnectUnigrams() StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		err := EachTokenLM(ctx, in, func(lm *LanguageModel, tokens ...T) error {
			for _, t := range tokens {
				lm.AddUnigram(t.Tokens[0])
			}
			if err := SendTokens(ctx, out, tokens...); err != nil {
				return fmt.Errorf("connect unigrams: %v", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("connect unigrams: %v", err)
		}
		return nil
	}
}

// ConnectLM connects the language model to the tokens.
func ConnectLM(ngrams FreqList) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		err := EachTokenGroup(ctx, in, func(group string, tokens ...T) error {
			// Add a new Language model to each token of the same group.
			lm := &LanguageModel{Ngrams: ngrams}
			for i := range tokens {
				tokens[i].LM = lm
			}
			if err := SendTokens(ctx, out, tokens...); err != nil {
				return fmt.Errorf("connect language model: %v", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("connect language model: %v", err)
		}
		return nil
	}
}

// ConnectRankings connects the tokens of the input stream with their
// respective rankings.
func ConnectRankings(lr *ml.LR, fs FeatureSet, n int) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		var lfid, ltid string // last file id and last token id
		var tokens []T
		err := EachToken(ctx, in, func(t T) error {
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
	}
}

func connectRankings(lr *ml.LR, fs FeatureSet, n int, tokens []T) T {
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
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		blen := 1024
		buf := make([]T, 0, blen)
		err := EachToken(ctx, in, func(t T) error {
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
	}
}

func connectCorrections(lr *ml.LR, fs FeatureSet, nocr int, tokens []T) {
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
