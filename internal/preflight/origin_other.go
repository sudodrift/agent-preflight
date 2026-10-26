//go:build !linux && !darwin

package preflight

func readOrigins(path string) []Origin { return nil }
