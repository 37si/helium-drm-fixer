//go:build !linux && !darwin && !windows

package helium

func platformOverride(string) (Install, bool) { return Install{}, false }

func find() (Install, error) { return Install{}, ErrNotFound }

func exe(Install) string { return "" }

func installed(Install) []CDM { return nil }

func root(Install) (string, error) { return "", ErrNotFound }

func target(Install, string) (string, string, error) { return "", "", ErrNotFound }

func writeHint(string, string) error { return nil }
