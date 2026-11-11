package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hdf/internal/browser"
)

const FileName = "config.cfg"

type Settings struct {
	Browser  string
	Helium   string
	Repo     string
	Interval string
	Keep     *int
	Newest   *bool
	Fresh    *bool
	NoBackup *bool
	External *bool
}

func Path(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, FileName)
}

func Load(path string) (Settings, error) {
	var settings Settings

	if path == "" {
		return settings, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return settings, nil
		}
		return settings, fmt.Errorf("read %s: %w", path, err)
	}

	if err := parse(raw, &settings); err != nil {
		return settings, fmt.Errorf("%s: %w", path, err)
	}

	if err := settings.validate(); err != nil {
		return settings, fmt.Errorf("%s: %w", path, err)
	}

	return settings, nil
}

func parse(raw []byte, settings *Settings) error {
	for number, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("line %d: want key = value, got %q", number+1, line)
		}

		if err := assign(strings.ToLower(strings.TrimSpace(key)), unquote(value), settings); err != nil {
			return fmt.Errorf("line %d: %w", number+1, err)
		}
	}

	return nil
}

func unquote(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		return value[1 : len(value)-1]
	}
	return value
}

func assign(key, value string, settings *Settings) error {
	switch strings.ReplaceAll(key, "_", "-") {
	case "browser":
		settings.Browser = value
	case "helium", "helium-path":
		settings.Helium = value
	case "repo":
		settings.Repo = value
	case "interval":
		settings.Interval = value
	case "keep":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("keep: %w", err)
		}
		settings.Keep = &n
	case "newest":
		return setBool(value, &settings.Newest, key)
	case "fresh":
		return setBool(value, &settings.Fresh, key)
	case "no-backup":
		return setBool(value, &settings.NoBackup, key)
	case "external":
		return setBool(value, &settings.External, key)
	default:
		return fmt.Errorf("unknown setting %q", key)
	}

	return nil
}

func setBool(value string, target **bool, key string) error {
	switch strings.ToLower(value) {
	case "true", "yes", "on", "1":
		on := true
		*target = &on
	case "false", "no", "off", "0":
		off := false
		*target = &off
	default:
		return fmt.Errorf("%s: want true or false, got %q", key, value)
	}

	return nil
}

func (s Settings) validate() error {
	if s.Browser != "" {
		if _, ok := browser.Parse(s.Browser); !ok {
			return fmt.Errorf("unknown browser %q", s.Browser)
		}
	}

	if s.Interval != "" {
		if _, err := time.ParseDuration(s.Interval); err != nil {
			return fmt.Errorf("interval %q: %w", s.Interval, err)
		}
	}

	if s.Keep != nil && *s.Keep < 0 {
		return errors.New("keep cannot be negative")
	}

	return nil
}

func (s Settings) PollInterval() (time.Duration, bool) {
	if s.Interval == "" {
		return 0, false
	}

	value, err := time.ParseDuration(s.Interval)
	if err != nil {
		return 0, false
	}

	return value, true
}
