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

// EachTokenInDocument iterates over the tokens grouping them together based on
// their language models. The given callback function is called for
// each group of tokens.  This function assumes that the tokens are
// connected with a language model.
func EachTokenInDocument(ctx context.Context, in <-chan T, f func(*Document, ...T) error) error {
	var doc *Document
	var tokens []T
	err := EachToken(ctx, in, func(t T) error {
		if doc == nil {
			doc = t.Document
		}
		if doc != t.Document {
			if err := f(doc, tokens...); err != nil {
				return fmt.Errorf("each token language model: %v", err)
			}
			tokens = tokens[0:0] // Clear token array.
			doc = t.Document
			return nil
		}
		tokens = append(tokens, t)
		return nil
	})
	// Handle last group of tokens.
	if len(tokens) != 0 {
		if err := f(doc, tokens...); err != nil {
			return fmt.Errorf("each token language model: %v", err)
		}
	}
	if err != nil {
		return fmt.Errorf("each token language model: %v", err)
	}
	return nil
}

// EachLine calls the given callback function for each line.
func EachLine(ctx context.Context, in <-chan T, f func([]T) error) error {
	var ts []T
	err := EachToken(ctx, in, func(t T) error {
		ts = append(ts, t)
		if t.EOL {
			if err := f(ts); err != nil {
				return fmt.Errorf("each line: %v", err)
			}
			ts = ts[0:0]
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("each line: %v", err)
	}
	if len(ts) != 0 {
		return fmt.Errorf("each line: missing end-of-line marker")
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
			// We need to handle end of line markers in a special way.  In
			// order to make sure that they are not removed even if they are
			// empty after normalization, we make them long
			// enough to not be removed (end of line markers are relevant for
			// mrg training, so a length of 1 is sufficient).
			if t.EOL && t.Tokens[0] == "" {
				t.Tokens[0] = "$"
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
			interp, ok := t.Document.Profile[t.Tokens[0]]
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

// AddShortTokensToProfile returns a stream function that adds fake
// profiler interpretation for short tokens into the token's profile.
// Short tokens are tokens with less than or equal to max unicode runes.
func AddShortTokensToProfile(max int) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		err := EachTokenInDocument(ctx, in, func(d *Document, ts ...T) error {
			for i, t := range ts {
				if utf8.RuneCountInString(t.Tokens[0]) > max {
					continue
				}
				if _, ok := t.Document.Profile[t.Tokens[0]]; ok {
					continue
				}
				ts[i].Document.Profile[t.Tokens[0]] = gofiler.Interpretation{
					N:   1,
					OCR: t.Tokens[0],
					Candidates: []gofiler.Candidate{
						{
							Suggestion: t.Tokens[0],
							Modern:     t.Tokens[0],
							Dict:       "short-split-tokens",
						},
					},
				}
			}
			return SendTokens(ctx, out, ts...)
		})
		if err != nil {
			return fmt.Errorf("add short tokens to profile: %v", err)
		}
		return nil
	}
}

// ConnectCandidates returns a stream function that connects tokens
// with their respective candidates to the stream.  Tokens with no
// candidates or tokens with only a modern interpretation are filtered
// from the stream.
func ConnectSplitCandidates() StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		err := EachToken(ctx, in, func(t T) error {
			interp, ok := t.Document.Profile[t.Tokens[0]]
			// Remove token with no candiates
			if !ok {
				return nil
			}
			if len(interp.Candidates) == 0 {
				return nil
			}
			split := t.Payload.(Split)
			split.Candidates = interp.Candidates
			for i := range split.Tokens {
				if interp, ok := t.Document.Profile[split.Tokens[i].Tokens[0]]; ok {
					split.Tokens[i].Payload = interp.Candidates
				}
			}
			t.Payload = split
			return SendTokens(ctx, out, t)
		})
		if err != nil {
			return fmt.Errorf("connect split candidates: %v", err)
		}
		return nil
	}
}

// ConnectProfile returns a stream function that connects the tokens with the profile.
func ConnectProfile(profile gofiler.Profile) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		return EachTokenInDocument(ctx, in, func(d *Document, tokens ...T) error {
			d.Profile = profile
			d.ocrPats = profile.GlobalOCRPatterns()
			return SendTokens(ctx, out, tokens...)
		})
	}
}

// ConnectUnigrams adds the unigrams to the tokens's language model.
func ConnectUnigrams() StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		err := EachTokenInDocument(ctx, in, func(lm *Document, tokens ...T) error {
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

// ConnectLanguageModel connects the document of the tokens to a language model.
func ConnectLanguageModel(lm map[string]*FreqList) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		return EachTokenInDocument(ctx, in, func(d *Document, tokens ...T) error {
			d.LM = lm
			if err := SendTokens(ctx, out, tokens...); err != nil {
				return fmt.Errorf("connect language model: %v", err)
			}
			return nil
		})
	}
}

// ConnectRankings connects the tokens of the input stream with their
// respective rankings.
func ConnectRankings(p ml.Predictor, fs FeatureSet, n int) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		var lfid, ltid string // last file id and last token id
		var tokens []T
		err := EachToken(ctx, in, func(t T) error {
			if t.File != lfid || t.ID != ltid {
				if len(tokens) > 0 {
					tmp := connectRankings(p, fs, n, tokens)
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
			t := connectRankings(p, fs, n, tokens)
			if err := SendTokens(ctx, out, t); err != nil {
				return err
			}
		}
		return nil
	}
}

func connectRankings(p ml.Predictor, fs FeatureSet, n int, tokens []T) T {
	var xs []float64
	// calculate feature values
	for _, token := range tokens {
		xs = fs.Calculate(xs, token, n)
	}
	// calculate prediction probabilities
	xmat := mat.NewDense(len(tokens), len(xs)/len(tokens), xs)
	probs := p.Predict(xmat)
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
func ConnectCorrections(p ml.Predictor, fs FeatureSet, n int) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		blen := 1024
		buf := make([]T, 0, blen)
		err := EachToken(ctx, in, func(t T) error {
			if len(buf) >= blen {
				connectCorrections(p, fs, n, buf)
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
			connectCorrections(p, fs, n, buf)
			if err := SendTokens(ctx, out, buf...); err != nil {
				return fmt.Errorf("connectCorrections: %v", err)
			}
		}
		return nil
	}
}

func connectCorrections(p ml.Predictor, fs FeatureSet, nocr int, tokens []T) {
	xs := make([]float64, 0, len(tokens)*len(fs))
	for _, t := range tokens {
		xs = fs.Calculate(xs, t, nocr)
	}
	x := mat.NewDense(len(tokens), len(xs)/len(tokens), xs)
	ps := p.Predict(x)
	for i := range tokens {
		tokens[i].Payload = Correction{
			Candidate: tokens[i].Payload.([]Ranking)[0].Candidate,
			Conf:      ps.AtVec(i),
		}
	}
}

func ConnectMergesWithGT() StreamFunc {
	samegt := func(ts []T) bool {
		for i := 1; i < len(ts); i++ {
			if ts[0].GT() != ts[i].GT() {
				return false
			}
		}
		return true
	}

	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		var ts []T
		err := EachLine(ctx, in, func(line []T) error {
			for i := 0; i < len(line); i++ {
				ts = ts[0:0]
				for j := len(line); j > i+1; j-- {
					ts = append(ts, makeMRGToken(line[i:j]))
				}
				// Longest merges are in front of the slice.
				for i := range ts {
					if samegt(ts[i].Payload.(Split).Tokens) {
						split := ts[i].Payload.(Split)
						split.Valid = true
						ts[i].Payload = split
						break
					}
				}
				if err := SendTokens(ctx, out, ts...); err != nil {
					return fmt.Errorf("connect merges with gt: %v", err)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("connect merges with gt: %v", err)
		}
		return nil
	}
}

// ts is not empty!
func makeMRGToken(ts []T) T {
	// Make a new copy of the first token;
	// We need to copy all the internal arrays and slices.
	t := ts[0]
	t.Tokens = make([]string, len(ts[0].Tokens))
	copy(t.Tokens, ts[0].Tokens)
	t.Chars = make(Chars, len(ts[0].Chars))
	copy(t.Chars, ts[0].Chars)
	t.Payload = Split{Tokens: make([]T, len(ts))}
	copy(t.Payload.(Split).Tokens, ts)

	for i := 1; i < len(ts); i++ {
		t.ID += "+" + ts[i].ID
		t.Chars = append(t.Chars, ts[i].Chars...)
		t.EOL = t.EOL || ts[i].EOL
		t.SOL = t.SOL || ts[i].SOL
		for j := range ts[i].Tokens {
			if j == 0 || !strings.HasSuffix(t.Tokens[j], ts[i].Tokens[j]) {
				t.Tokens[j] += ts[i].Tokens[j]
			}
			if j == len(ts[i].Tokens)-1 && ts[i].Tokens[j] == "" {
				t.Tokens[j] += "@"
			}
		}
	}
	return t
}
