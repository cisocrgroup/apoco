package apoco

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func sendtoks(ts ...Token) StreamFunc {
	return func(ctx context.Context, in <-chan Token, out chan<- Token) error {
		return SendTokens(ctx, out, ts...)
	}
}

func readtoks(ts *[]Token) StreamFunc {
	return func(ctx context.Context, in <-chan Token, out chan<- Token) error {
		return EachToken(ctx, in, func(t Token) error {
			*ts = append(*ts, t)
			return nil
		})
	}
}

func counttoks(cnt *int) StreamFunc {
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

func TestFilterBad(t *testing.T) {
	for _, tc := range []struct {
		test []Token
		want string
	}{
		{mktoks("a|b", "a|b|c"), "a|b|c"},
		{mktoks("a|b|c", "a|b", "a|b|c"), "a|b|c a|b|c"},
	} {
		t.Run(fmttoks(tc.test...), func(t *testing.T) {
			var got []Token
			err := Pipe(context.Background(),
				sendtoks(tc.test...), FilterBad(3), readtoks(&got))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if got := fmttoks(got...); got != tc.want {
				t.Fatalf("expected %s; got %s", tc.want, got)
			}
		})
	}
}

func TestFilterShort(t *testing.T) {
	for _, tc := range []struct {
		test []Token
		want string
	}{
		{mktoks("aaa|bbb", "aaaa|b|c"), "aaaa|b|c"},
		{mktoks("aaaa|b|c", "aa|bb", "aaaa|b|c"), "aaaa|b|c aaaa|b|c"},
	} {
		t.Run(fmttoks(tc.test...), func(t *testing.T) {
			var got []Token
			err := Pipe(context.Background(),
				sendtoks(tc.test...), FilterShort(4), readtoks(&got))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if got := fmttoks(got...); got != tc.want {
				t.Fatalf("expected %s; got %s", tc.want, got)
			}
		})
	}
}

func TestCombine(t *testing.T) {
	for _, tc := range []struct {
		test []Token
		want string
	}{
		{mktoks("A|B|C", "D|E", "F|G|H"), "a|b|c f|g|h"},
		//{mktoks("aaaa|b|c", "aa|bb", "aaaa|b|c"), "aaaa|b|c aaaa|b|c"},
	} {
		t.Run(fmttoks(tc.test...), func(t *testing.T) {
			var got []Token
			ctx := context.Background()
			err := Pipe(
				ctx,
				sendtoks(tc.test...),
				Combine(ctx, FilterBad(3), Normalize),
				readtoks(&got))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if got := fmttoks(got...); got != tc.want {
				t.Fatalf("expected %s; got %s", tc.want, got)
			}
		})
	}
}
