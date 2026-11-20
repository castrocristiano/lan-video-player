package safefs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func setupSafeFSTestRoot(t *testing.T) (*Root, string, string) {
	t.Helper()
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
	if err := os.WriteFile(filepath.Join(mediaDir, ".secret.mp4"), []byte("hidden"), 0o644); err != nil {
		t.Fatal(err)
	}
	outsideSecret := filepath.Join(outsideDir, "passwd.txt")
	if err := os.WriteFile(outsideSecret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	_ = os.Symlink(outsideSecret, filepath.Join(mediaDir, "Movies", "escape.mp4"))
	_ = os.Symlink(validVideo, filepath.Join(mediaDir, "Movies", "internal_link.mp4"))

	root, err := NewRoot(mediaDir)
	if err != nil {
		t.Fatalf("NewRoot failed: %v", err)
	}
	return root, validVideo, outsideSecret
}

func TestRootResolveAndTraversal(t *testing.T) {
	root, _, _ := setupSafeFSTestRoot(t)

	_, normRel, err := root.Resolve("Movies/clip.mp4")
	if err != nil || normRel != "Movies/clip.mp4" {
		t.Fatalf("Resolve valid path failed: err=%v norm=%q", err, normRel)
	}
	if _, normRoot, err := root.Resolve(""); err != nil || normRoot != "" {
		t.Fatalf("Resolve root failed: err=%v norm=%q", err, normRoot)
	}
	if _, _, err := root.Resolve("Movies/internal_link.mp4"); err != nil {
		t.Fatalf("Resolve internal symlink failed: %v", err)
	}
	if _, _, err := root.Resolve("Movies/missing.mp4"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	var nilRoot *Root
	if _, _, err := nilRoot.Resolve("a"); !errors.Is(err, ErrForbiddenPath) {
		t.Errorf("expected ErrForbiddenPath on nil root")
	}

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
}

func TestRootReadDirAndReadOnly(t *testing.T) {
	root, validVideo, outsideSecret := setupSafeFSTestRoot(t)

	entries, _, err := root.ReadDir("Movies")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, e := range entries {
		if e.Name() == "escape.mp4" {
			t.Errorf("ReadDir leaked external symlink")
		}
	}
	if _, _, err := root.ReadDir("Movies/clip.mp4"); !errors.Is(err, ErrNotDirectory) {
		t.Errorf("expected ErrNotDirectory on ReadDir of file, got %v", err)
	}

	f, info, _, err := root.OpenReadOnly("Movies/clip.mp4")
	if err != nil {
		t.Fatalf("OpenReadOnly failed: %v", err)
	}
	defer f.Close()
	if info.Size() != int64(len("fake-mp4-data")) {
		t.Errorf("unexpected file size: %d", info.Size())
	}
	if _, writeErr := f.Write([]byte("mutate")); writeErr == nil {
		t.Errorf("expected write to read-only file descriptor to fail")
	}

	if rel, ok := root.RelFromRoot(validVideo); !ok || rel != "Movies/clip.mp4" {
		t.Errorf("RelFromRoot validVideo failed: rel=%q ok=%v", rel, ok)
	}
	if rel, ok := root.RelFromRoot(root.CanonicalPath()); !ok || rel != "" {
		t.Errorf("RelFromRoot root failed: rel=%q ok=%v", rel, ok)
	}
	if _, ok := root.RelFromRoot(outsideSecret); ok {
		t.Errorf("RelFromRoot should fail for outside path")
	}
	if info, _, err := root.Stat("Movies/clip.mp4"); err != nil || info.IsDir() {
		t.Errorf("Stat validVideo failed: %v", err)
	}
}

func TestNewRootValidationEdges(t *testing.T) {
	_, validVideo, _ := setupSafeFSTestRoot(t)
	if _, err := NewRoot(""); err == nil {
		t.Errorf("expected error for empty NewRoot")
	}
	if _, err := NewRoot("/nonexistent/safefs/test/dir"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if _, err := NewRoot(validVideo); !errors.Is(err, ErrNotDirectory) {
		t.Errorf("expected ErrNotDirectory, got %v", err)
	}
	if IsHiddenName(".") || !IsHiddenName(".env") {
		t.Errorf("unexpected IsHiddenName result")
	}
}
