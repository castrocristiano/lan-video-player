package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseValidAndInvalidMediaDirs(t *testing.T) {
	tmp := t.TempDir()
	moviesDir := filepath.Join(tmp, "Filmes")
	seriesDir := filepath.Join(tmp, "Series")
	if err := os.MkdirAll(moviesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(seriesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	cfg, err := Parse(
		[]string{
			"--addr", "127.0.0.1:9090",
			"--media", moviesDir,
			"--media", seriesDir,
			"--browse-base", tmp,
		},
		func(string) string { return "" },
		&stderr,
	)
	if err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
	if cfg.Addr != "127.0.0.1:9090" {
		t.Errorf("expected Addr 127.0.0.1:9090, got %q", cfg.Addr)
	}
	if len(cfg.MediaDirs) != 2 {
		t.Fatalf("expected 2 media dirs, got %d", len(cfg.MediaDirs))
	}

	// Non-existent media directory must fail startup validation (FR-002)
	missingDir := filepath.Join(tmp, "NaoExiste")
	_, err = Parse(
		[]string{"--media", missingDir},
		func(string) string { return "" },
		&stderr,
	)
	if err == nil || !strings.Contains(err.Error(), "NaoExiste") {
		t.Fatalf("expected startup failure mentioning invalid path NaoExiste, got: %v", err)
	}

	// File instead of directory must also fail startup validation (FR-002)
	fileNotDir := filepath.Join(tmp, "video.mp4")
	if err := os.WriteFile(fileNotDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Parse(
		[]string{"--media", fileNotDir},
		func(string) string { return "" },
		&stderr,
	)
	if err == nil {
		t.Fatalf("expected startup failure when --media points to a regular file")
	}
}

func TestStoreAtomicLoadAndSave(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config", "library.json")
	store := NewStore(cfgPath)

	// Initial Load before file exists returns empty v1 config
	initial, err := store.Load()
	if err != nil {
		t.Fatalf("Load on missing file failed: %v", err)
	}
	if initial.Version != CurrentSchemaVersion || len(initial.Roots) != 0 {
		t.Fatalf("unexpected initial config: %+v", initial)
	}

	// Save two roots atomically
	entries := []SavedRootEntry{
		{Path: "/media/Filmes", Label: "Filmes"},
		{Path: "/media/Cursos", Label: "Cursos"},
	}
	if err := store.Save(entries); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reload and verify persistence (FR-019)
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load after Save failed: %v", err)
	}
	if loaded.Version != 1 || len(loaded.Roots) != 2 {
		t.Fatalf("expected 2 saved roots, got: %+v", loaded)
	}
	if loaded.Roots[0].Path != "/media/Filmes" || loaded.Roots[1].Path != "/media/Cursos" {
		t.Errorf("unexpected saved root paths: %+v", loaded.Roots)
	}
}
