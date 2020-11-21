package apoco

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func sendtoks(ts ...Token) StreamFuncX {
	return func(ctx context.Context, in <-chan Token, out chan<- Token) error {
		return SendTokens(ctx, out, ts...)
	}
}

func readtoks(ts *[]Token) StreamFuncX {
	return func(ctx context.Context, in <-chan Token, out chan<- Token) error {
		return EachToken(ctx, in, func(t Token) error {
			*ts = append(*ts, t)
			return nil
		})
	}
}

func counttoks(cnt *int) StreamFuncX {
	return func(ctx context.Context, in <-chan Token, out chan<- Token) error {
		return EachToken(ctx, in, func(_ Token) error {
			*cnt++
			return nil
		})
	}
}

func mktoks(ts ...string) []Token {
	ret := make([]Token, len(ts))
	for i, t := range ts {
		ret[i] = Token{Tokens: strings.Split(t, "|"), ID: strconv.Itoa(i + 1)}
	}
	return ret
}

func fmttoks(ts ...Token) string {
	strs := make([]string, len(ts))
	for i, t := range ts {
		strs[i] = t.String()
	}
	return strings.Join(strs, " ")
}

func TestCountTokens(t *testing.T) {
	for _, tc := range []struct {
		tokens []Token
		want   int
	}{
		{nil, 0},
		{make([]Token, 100), 100},
		{make([]Token, 10), 10},
	} {
		t.Run(fmt.Sprintf("count %d", tc.want), func(t *testing.T) {
			var count int
			err := Pipe(context.Background(),
				sendtoks(tc.tokens...), counttoks(&count))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if count != tc.want {
				t.Fatalf("expected %d; got %d", tc.want, count)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	for _, tc := range []struct {
		test []Token
		want string
	}{
		{mktoks(",A|B."), "a|b"},
		{mktoks(",A|B C", "x|y z"), "a|b_c x|y_z"},
	} {
		t.Run(fmttoks(tc.test...), func(t *testing.T) {
			var got []Token
			err := Pipe(context.Background(),
				sendtoks(tc.test...), Normalize, readtoks(&got))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if got := fmttoks(got...); got != tc.want {
				t.Fatalf("expected %s; got %s", tc.want, got)
			}
		})
	}
}
