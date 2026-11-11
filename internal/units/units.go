package units

import (
	"fmt"
	"time"
)

func Since(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	elapsed := time.Since(t)

	switch {
	case elapsed < 45*time.Second:
		return "just now"
	case elapsed < 90*time.Second:
		return "a minute ago"
	case elapsed < time.Hour:
		return plural(int(elapsed.Minutes()+0.5), "minute")
	case elapsed < 24*time.Hour:
		return plural(int(elapsed.Hours()+0.5), "hour")
	case elapsed < 30*24*time.Hour:
		return plural(int(elapsed.Hours()/24+0.5), "day")
	case elapsed < 365*24*time.Hour:
		return plural(int(elapsed.Hours()/(24*30)+0.5), "month")
	default:
		return plural(int(elapsed.Hours()/(24*365)+0.5), "year")
	}
}

func plural(n int, unit string) string {
	if n <= 1 {
		return "1 " + unit + " ago"
	}
	return fmt.Sprintf("%d %ss ago", n, unit)
}

func Bytes(n int64) string {
	const step = 1024

	if n < step {
		return fmt.Sprintf("%d B", n)
	}

	value := float64(n)
	units := []string{"KB", "MB", "GB", "TB"}

	for _, unit := range units {
		value /= step
		if value < step {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}

	return fmt.Sprintf("%.1f PB", value/step)
}

func Duration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return d.Round(time.Second).String()
	}
}
