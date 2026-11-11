package units

import (
	"testing"
	"time"
)

func TestSince(t *testing.T) {
	now := time.Now()

	cases := []struct {
		ago  time.Duration
		want string
	}{
		{5 * time.Second, "just now"},
		{70 * time.Second, "a minute ago"},
		{5 * time.Minute, "5 minutes ago"},
		{90 * time.Minute, "2 hours ago"},
		{30 * time.Hour, "1 day ago"},
		{50 * time.Hour, "2 days ago"},
		{10 * 24 * time.Hour, "10 days ago"},
		{90 * 24 * time.Hour, "3 months ago"},
		{800 * 24 * time.Hour, "2 years ago"},
	}

	for _, c := range cases {
		if got := Since(now.Add(-c.ago)); got != c.want {
			t.Fatalf("Since(-%v) = %q, want %q", c.ago, got, c.want)
		}
	}

	if got := Since(time.Time{}); got != "unknown" {
		t.Fatalf("Since(zero) = %q", got)
	}
}

func TestBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{512, "512 B"},
		{1536, "1.5 KB"},
		{18 << 20, "18.0 MB"},
		{3 << 30, "3.0 GB"},
	}

	for _, c := range cases {
		if got := Bytes(c.in); got != c.want {
			t.Fatalf("Bytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{90 * time.Second, "1m30s"},
	}

	for _, c := range cases {
		if got := Duration(c.in); got != c.want {
			t.Fatalf("Duration(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
