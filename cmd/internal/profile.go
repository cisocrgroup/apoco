package internal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git.sr.ht/~flobar/apoco/pkg/apoco"
	"github.com/finkf/gofiler"
)

// ConnectProfile generates the profile by running the profiler or reads
// the profile from the cache and connects the profile with the tokens.
func ConnectProfile(c *Config, suffix string) apoco.StreamFunc {
	return func(ctx context.Context, in <-chan apoco.T, out chan<- apoco.T) error {
		return apoco.EachTokenInDocument(ctx, in, func(doc *apoco.Document, ts ...apoco.T) error {
			profile, err := readProfile(ctx, c, doc.Group, suffix, ts)
			if err != nil {
				return fmt.Errorf("connect profile: %v", err)
			}
			doc.Profile = profile
			if err := apoco.SendTokens(ctx, out, ts...); err != nil {
				return fmt.Errorf("connect profile: %v", err)
			}
			return nil
		})
	}
}

func readProfile(ctx context.Context, c *Config, group, suffix string, ts []apoco.T) (gofiler.Profile, error) {
	path, ok := profilerCachePath(group, suffix)
	if ok && c.Cache {
		profile, err := apoco.ReadProfile(path)
		if err == nil { // If an error occurs, run the profiler.
			apoco.Log("read %d profile tokens from %s", len(profile), path)
			return profile, nil
		}
	}
	var merged []apoco.T
	for _, t := range ts {
		if split, ok := t.Payload.(apoco.Split); ok {
			merged = append(merged, split.Tokens...)
		}
	}
	ts = append(ts, merged...)
	profile, err := apoco.RunProfiler(ctx, c.Profiler.Exe, c.Profiler.Config, ts...)
	if err != nil {
		return nil, err
	}
	if ok && c.Cache {
		apoco.Log("writing %d profile tokens to %s", len(profile), path)
		_ = apoco.WriteProfile(path, profile)
	}
	return profile, nil
}

func profilerCachePath(group, suffix string) (string, bool) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", false
	}
	cacheDir := filepath.Join(dir, "apoco")
	_ = os.MkdirAll(cacheDir, 0755)
	return filepath.Join(cacheDir, strings.ReplaceAll(group, "/", "-")+suffix), true
}
