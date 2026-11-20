package safefs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRootResolveAndTraversalProtection(t *testing.T) {
	tmp := t.TempDir()
	mediaDir := filepath.Join(tmp, "media")
	outsideDir := filepath.Join(tmp, "outside")
	if err := os.MkdirAll(filepath.Join(mediaDir, "Movies"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatal(err)
	}

	validVideo := filepath.Join(mediaDir, "Movies", "clip.mp4")
	if err := os.WriteFile(validVideo, []byte("fake-mp4-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	hiddenFile := filepath.Join(mediaDir, ".secret.mp4")
	if err := os.WriteFile(hiddenFile, []byte("hidden"), 0o644); err != nil {
		t.Fatal(err)
	}
	outsideSecret := filepath.Join(outsideDir, "passwd.txt")
	if err := os.WriteFile(outsideSecret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside mediaDir pointing outside to outsideSecret
	escapeLink := filepath.Join(mediaDir, "Movies", "escape.mp4")
	if err := os.Symlink(outsideSecret, escapeLink); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside mediaDir pointing to a valid file inside mediaDir
	internalLink := filepath.Join(mediaDir, "Movies", "internal_link.mp4")
	if err := os.Symlink(validVideo, internalLink); err != nil {
		t.Fatal(err)
	}

	root, err := NewRoot(mediaDir)
	if err != nil {
		t.Fatalf("NewRoot failed: %v", err)
	}

	// 1. Valid path
	_, normRel, err := root.Resolve("Movies/clip.mp4")
	if err != nil {
		t.Fatalf("expected valid path to resolve, got: %v", err)
	}
	if normRel != "Movies/clip.mp4" {
		t.Fatalf("expected normalized rel 'Movies/clip.mp4', got %q", normRel)
	}

	// 2. Internal symlink inside root is allowed
	if _, _, err := root.Resolve("Movies/internal_link.mp4"); err != nil {
		t.Fatalf("expected internal symlink to resolve, got: %v", err)
	}

	// 3. Traversal attempts must return ErrForbiddenPath
	traversalCases := []string{
		"..",
		"../outside/passwd.txt",
		"Movies/../../outside/passwd.txt",
		"..\\outside\\passwd.txt",
		".secret.mp4",
		"Movies/.hidden/clip.mp4",
		"Movies/escape.mp4",
		"Movies/clip.mp4\x00.txt",
	}
	for _, tc := range traversalCases {
		if _, _, err := root.Resolve(tc); !errors.Is(err, ErrForbiddenPath) {
			t.Errorf("Resolve(%q): expected ErrForbiddenPath, got %v", tc, err)
		}
	}

	// 4. Non-existent path inside root must return ErrNotFound
	if _, _, err := root.Resolve("Movies/missing.mp4"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for missing file, got %v", err)
	}

	// 5. ReadDir filters out hidden files and external symlinks
	entries, _, err := root.ReadDir("Movies")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, e := range entries {
		if e.Name() == "escape.mp4" {
			t.Errorf("ReadDir leaked external symlink escape.mp4")
		}
	}

	// 6. OpenReadOnly enforces read-only file descriptor (Write must fail)
	f, info, _, err := root.OpenReadOnly("Movies/clip.mp4")
	if err != nil {
		t.Fatalf("OpenReadOnly failed: %v", err)
	}
	defer f.Close()
	if info.Size() != int64(len("fake-mp4-data")) {
		t.Errorf("unexpected file size: %d", info.Size())
	}
	if _, writeErr := f.Write([]byte("mutate")); writeErr == nil {
		t.Errorf("expected write to read-only file descriptor to fail, but it succeeded")
	}

	// 7. RelFromRoot and Stat and NewRoot edge cases
	if rel, ok := root.RelFromRoot(validVideo); !ok || rel != "Movies/clip.mp4" {
		t.Errorf("RelFromRoot validVideo failed: rel=%q ok=%v", rel, ok)
	}
	if _, ok := root.RelFromRoot(outsideSecret); ok {
		t.Errorf("RelFromRoot should return false for outside path")
	}
	if info, norm, err := root.Stat("Movies/clip.mp4"); err != nil || info.IsDir() || norm != "Movies/clip.mp4" {
		t.Errorf("Stat validVideo failed: err=%v", err)
	}
	if _, err := NewRoot(""); err == nil {
		t.Errorf("expected error for empty NewRoot path")
	}
	if _, err := NewRoot( filepath.Join(tmp, "missing_dir") ); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for missing NewRoot dir, got %v", err)
	}
	if _, err := NewRoot(validVideo); !errors.Is(err, ErrNotDirectory) {
		t.Errorf("expected ErrNotDirectory when NewRoot points to file, got %v", err)
	}
	if _, _, _, err := root.OpenReadOnly("Movies"); err == nil {
		t.Errorf("expected OpenReadOnly on directory to fail")
	}
}
