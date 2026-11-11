package gmp

import (
	"context"
	"os"
	"testing"
	"time"

	"charm.land/log/v2"

	"hdf/internal/cdm"
)

func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func versionOf(dir string) (string, error) { return cdm.Version(dir) }

func TestLiveDownload(t *testing.T) {
	if os.Getenv("HDF_LIVE") == "" {
		t.Skip("set HDF_LIVE=1 to talk to Mozilla")
	}

	client, err := New(t.TempDir(), log.New(os.Stderr))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := contextWithTimeout(2 * time.Minute)
	defer cancel()

	release, err := client.Latest(ctx, "")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}

	t.Logf("release %+v", release)

	if release.Version == "" || release.Hash == "" {
		t.Fatalf("incomplete release: %+v", release)
	}

	dir, err := client.Fetch(ctx, release, nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	t.Logf("unpacked into %s", dir)

	if version, err := versionOf(dir); err != nil || version != release.Version {
		t.Fatalf("version = %q, %v", version, err)
	}

	again, err := client.Fetch(ctx, release, nil)
	if err != nil || again != dir {
		t.Fatalf("cached fetch = %q, %v", again, err)
	}
}
