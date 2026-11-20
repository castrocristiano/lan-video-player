package main

import (
	"testing"
)

func TestRunVersionAndInvalidMedia(t *testing.T) {
	if code := run([]string{"--version"}, func(string) string { return "" }); code != 0 {
		t.Errorf("expected exit code 0 for --version, got %d", code)
	}
	if code := run([]string{"--media", "/nonexistent/path/for/lvp/test"}, func(string) string { return "" }); code != 1 {
		t.Errorf("expected exit code 1 for invalid --media path, got %d", code)
	}
}
