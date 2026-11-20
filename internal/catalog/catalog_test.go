package catalog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristiano/lan-video-player/internal/config"
)

type testCatalogEnv struct {
	tmp        string
	browseBase string
	cliMovies  string
	uiSeries   string
	store      *config.Store
	cat        *Catalog
}

func mustWriteFile(t *testing.T, p string, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setupCatalogEnv(t *testing.T) *testCatalogEnv {
	t.Helper()
	tmp := t.TempDir()
	browseBase := filepath.Join(tmp, "media")
	cliMovies := filepath.Join(browseBase, "Filmes")
	uiSeries := filepath.Join(browseBase, "Series", "Temporada 1")
	if err := os.MkdirAll(cliMovies, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(uiSeries, 0o755); err != nil {
		t.Fatal(err)
	}

	mustWriteFile(t, filepath.Join(cliMovies, "Matrix.mp4"), "video-matrix")
	mustWriteFile(t, filepath.Join(cliMovies, "Matrix.jpg"), "fake-jpeg-bytes")
	mustWriteFile(t, filepath.Join(cliMovies, "notes.txt"), "ignore me")
	mustWriteFile(t, filepath.Join(cliMovies, ".hidden.mp4"), "ignore hidden")

	ep1 := filepath.Join(uiSeries, "Episodio 01.mp4")
	ep2 := filepath.Join(uiSeries, "Episodio 02.webm")
	mustWriteFile(t, ep1, "ep1")
	mustWriteFile(t, ep2, "ep2")

	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-10 * time.Minute)
	_ = os.Chtimes(ep1, oldTime, oldTime)
	_ = os.Chtimes(ep2, newTime, newTime)

	mustWriteFile(t, filepath.Join(uiSeries, "Episodio 01.pt-BR.srt"), "1\r\n00:00:01,250 --> 00:00:04,500\r\nOlá Rede Local!\r\n")
	mustWriteFile(t, filepath.Join(uiSeries, "Episodio 01.en.vtt"), "WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nHello LAN!\n")
	mustWriteFile(t, filepath.Join(uiSeries, "Episodio 01.srt"), "1\n00:00:01,000 --> 00:00:02,000\nPadrao\n")

	store := config.NewStore(filepath.Join(tmp, "config", "library.json"))
	cat, err := New([]string{cliMovies}, browseBase, store)
	if err != nil {
		t.Fatalf("New catalog failed: %v", err)
	}

	return &testCatalogEnv{
		tmp:        tmp,
		browseBase: browseBase,
		cliMovies:  cliMovies,
		uiSeries:   uiSeries,
		store:      store,
		cat:        cat,
	}
}

func TestCatalogInitialRootsAndPicker(t *testing.T) {
	env := setupCatalogEnv(t)

	if !env.cat.BrowseBaseEnabled() {
		t.Fatalf("expected browseBase enabled")
	}
	if env.cat.BrowseBaseLabel() == "" {
		t.Fatalf("expected non-empty browseBase label")
	}

	roots := env.cat.Roots()
	if len(roots) != 1 {
		t.Fatalf("unexpected initial roots count: %d", len(roots))
	}

	if _, err := env.cat.GetRoot(""); err != nil {
		t.Fatalf("GetRoot empty failed: %v", err)
	}

	pickerRes, err := env.cat.ListBrowseBaseDirs("")
	if err != nil {
		t.Fatalf("ListBrowseBaseDirs failed: %v", err)
	}
	if len(pickerRes.Directories) != 2 {
		t.Fatalf("expected 2 picker directories, got %d", len(pickerRes.Directories))
	}
}

func TestCatalogAddReloadAndRemoveUIRoots(t *testing.T) {
	env := setupCatalogEnv(t)

	updatedRoots, err := env.cat.AddUIRoots([]string{"Series"})
	if err != nil {
		t.Fatalf("AddUIRoots failed: %v", err)
	}
	if len(updatedRoots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(updatedRoots))
	}

	// Idempotent add
	if _, err := env.cat.AddUIRoots([]string{"Series"}); err != nil {
		t.Fatalf("idempotent AddUIRoots failed: %v", err)
	}

	// Reload from store
	catReloaded, err := New([]string{env.cliMovies}, env.browseBase, env.store)
	if err != nil {
		t.Fatalf("reloaded catalog failed: %v", err)
	}
	if len(catReloaded.Roots()) != 2 {
		t.Fatalf("expected 2 reloaded roots")
	}

	seriesRootID := updatedRoots[1].ID
	if _, err := env.cat.RemoveUIRoot(updatedRoots[0].ID); err == nil {
		t.Errorf("expected removing CLI root to fail")
	}
	if _, err := env.cat.RemoveUIRoot("missing-root"); err == nil {
		t.Errorf("expected removing missing root to fail")
	}
	if _, err := env.cat.RemoveUIRoot(seriesRootID); err != nil {
		t.Fatalf("RemoveUIRoot failed: %v", err)
	}
}

func TestCatalogBrowseAndSorting(t *testing.T) {
	env := setupCatalogEnv(t)
	// Add second subfolder under Series so sortFolders exercises multiple entries
	if err := os.MkdirAll(filepath.Join(env.browseBase, "Series", "Temporada 2"), 0o755); err != nil {
		t.Fatal(err)
	}
	updatedRoots, err := env.cat.AddUIRoots([]string{"Series"})
	if err != nil {
		t.Fatal(err)
	}
	seriesRootID := updatedRoots[1].ID

	for _, sortOpt := range []string{"name_asc", "name_desc", "date_desc", "date_asc"} {
		if _, err := env.cat.Browse(seriesRootID, "Temporada 1", sortOpt); err != nil {
			t.Fatalf("Browse Temporada 1 (%s) failed: %v", sortOpt, err)
		}
		if _, err := env.cat.Browse(seriesRootID, "", sortOpt); err != nil {
			t.Fatalf("Browse root (%s) failed: %v", sortOpt, err)
		}
	}
}

func TestCatalogSearchSubtitlesAndThumbnails(t *testing.T) {
	env := setupCatalogEnv(t)
	updatedRoots, err := env.cat.AddUIRoots([]string{"Series"})
	if err != nil {
		t.Fatal(err)
	}
	seriesRootID := updatedRoots[1].ID
	sRoot := updatedRoots[1].SafeRoot()

	vttOut, err := ReadSubtitleAsWebVTT(sRoot, "Temporada 1/Episodio 01.pt-BR.srt")
	if err != nil || !strings.HasPrefix(string(vttOut), "WEBVTT\n\n") {
		t.Fatalf("ReadSubtitleAsWebVTT SRT failed: %v", err)
	}
	vttDirect, err := ReadSubtitleAsWebVTT(sRoot, "Temporada 1/Episodio 01.en.vtt")
	if err != nil || !strings.HasPrefix(string(vttDirect), "WEBVTT") {
		t.Fatalf("ReadSubtitleAsWebVTT VTT failed: %v", err)
	}
	if _, err := ReadSubtitleAsWebVTT(sRoot, "Temporada 1/Episodio 01.mp4"); err == nil {
		t.Errorf("expected non-subtitle file to be rejected")
	}

	if res, err := env.cat.Search("", "", "name_asc"); err != nil || res.Total != 0 {
		t.Errorf("empty search failed")
	}
	if res, err := env.cat.Search("episodio", seriesRootID, "date_asc"); err != nil || res.Total != 2 {
		t.Errorf("filtered search failed")
	}

	thumbBytes, thumbMime, err := env.cat.ResolveThumbnail(context.Background(), updatedRoots[0].ID, "Matrix.mp4", "", true)
	if err != nil || thumbMime != mimeImageJPEG || string(thumbBytes) != "fake-jpeg-bytes" {
		t.Errorf("ResolveThumbnail sidecar failed: %v", err)
	}
	if !RequiresTranscodeByDefault("a.avi") || RequiresTranscodeByDefault("a.mp4") {
		t.Errorf("unexpected RequiresTranscodeByDefault")
	}
}
