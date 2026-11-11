//go:build !darwin

package fixer

import "context"

func needsResign() bool { return false }

func resign(context.Context, string, string) error { return nil }
