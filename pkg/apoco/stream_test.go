package apoco

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func sendtoks(ts ...T) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		return SendTokens(ctx, out, ts...)
	}
}

func readtoks(ts *[]T) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		return EachToken(ctx, in, func(t T) error {
			*ts = append(*ts, t)
			return nil
		})
	}
}

func counttoks(cnt *int) StreamFunc {
	return func(ctx context.Context, in <-chan T, out chan<- T) error {
		return EachToken(ctx, in, func(_ T) error {
			*cnt++
			return nil
		})
	}
}

func mktoks(ts ...string) []T {
	ret := make([]T, len(ts))
	for i, t := range ts {
		ret[i] = T{Tokens: strings.Split(t, "|"), ID: strconv.Itoa(i + 1)}
	}
	return ret
}

func fmttoks(ts ...T) string {
	strs := make([]string, len(ts))
	for i, t := range ts {
		strs[i] = t.String()
	}
	return strings.Join(strs, " ")
}

func TestCountTokens(t *testing.T) {
	for _, tc := range []struct {
		tokens []T
		want   int
	}{
		{nil, 0},
		{make([]T, 100), 100},
		{make([]T, 10), 10},
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
		test []T
		want string
	}{
		{mktoks(",A|B."), "a|b"},
		{mktoks(",A|B C", "x|y z"), "a|b_c x|y_z"},
	} {
		t.Run(fmttoks(tc.test...), func(t *testing.T) {
			var got []T
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
		test []T
		want string
	}{
		{mktoks("a|b", "a|b|c"), "a|b|c"},
		{mktoks("a|b|c", "a|b", "a|b|c"), "a|b|c a|b|c"},
	} {
		t.Run(fmttoks(tc.test...), func(t *testing.T) {
			var got []T
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
		test []T
		want string
	}{
		{mktoks("aaa|bbb", "aaaa|b|c"), "aaaa|b|c"},
		{mktoks("aaaa|b|c", "aa|bb", "aaaa|b|c"), "aaaa|b|c aaaa|b|c"},
	} {
		t.Run(fmttoks(tc.test...), func(t *testing.T) {
			var got []T
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
		test []T
		want string
	}{
		{mktoks("A|B|C", "D|E", "F|G|H"), "a|b|c f|g|h"},
		//{mktoks("aaaa|b|c", "aa|bb", "aaaa|b|c"), "aaaa|b|c aaaa|b|c"},
	} {
		t.Run(fmttoks(tc.test...), func(t *testing.T) {
			var got []T
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
