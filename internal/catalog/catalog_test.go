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
	ep1        string
	ep2        string
	store      *config.Store
	cat        *Catalog
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

	if err := os.WriteFile(filepath.Join(cliMovies, "Matrix.mp4"), []byte("video-matrix"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cliMovies, "Matrix.jpg"), []byte("fake-jpeg-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cliMovies, "notes.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cliMovies, ".hidden.mp4"), []byte("ignore hidden"), 0o644); err != nil {
		t.Fatal(err)
	}

	ep1 := filepath.Join(uiSeries, "Episodio 01.mp4")
	ep2 := filepath.Join(uiSeries, "Episodio 02.webm")
	if err := os.WriteFile(ep1, []byte("ep1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ep2, []byte("ep2"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-10 * time.Minute)
	_ = os.Chtimes(ep1, oldTime, oldTime)
	_ = os.Chtimes(ep2, newTime, newTime)

	srtContent := "1\r\n00:00:01,250 --> 00:00:04,500\r\nOlá Rede Local!\r\n"
	if err := os.WriteFile(filepath.Join(uiSeries, "Episodio 01.pt-BR.srt"), []byte(srtContent), 0o644); err != nil {
		t.Fatal(err)
	}
	vttContent := "WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nHello LAN!\n"
	if err := os.WriteFile(filepath.Join(uiSeries, "Episodio 01.en.vtt"), []byte(vttContent), 0o644); err != nil {
		t.Fatal(err)
	}

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
		ep1:        ep1,
		ep2:        ep2,
		store:      store,
		cat:        cat,
	}
}

func TestCatalogRootsAndPicker(t *testing.T) {
	env := setupCatalogEnv(t)

	if !env.cat.BrowseBaseEnabled() || env.cat.BrowseBaseLabel() == "" {
		t.Fatalf("expected browseBase to be enabled with label")
	}

	roots := env.cat.Roots()
	if len(roots) != 1 || roots[0].Source != "cli" || roots[0].Removable {
		t.Fatalf("unexpected initial roots: %+v", roots)
	}

	// Single root lookup with empty ID
	if single, err := env.cat.GetRoot(""); err != nil || single.ID != roots[0].ID {
		t.Errorf("GetRoot('') failed: %v", err)
	}

	pickerRes, err := env.cat.ListBrowseBaseDirs("")
	if err != nil || len(pickerRes.Directories) != 2 {
		t.Fatalf("ListBrowseBaseDirs failed: err=%v, count=%d", err, len(pickerRes.Directories))
	}

	updatedRoots, err := env.cat.AddUIRoots([]string{"Series"})
	if err != nil || len(updatedRoots) != 2 {
		t.Fatalf("AddUIRoots failed: err=%v, count=%d", err, len(updatedRoots))
	}

	// Idempotent add
	updatedRoots, err = env.cat.AddUIRoots([]string{"Series"})
	if err != nil || len(updatedRoots) != 2 {
		t.Fatalf("expected idempotent AddUIRoots, got err=%v count=%d", err, len(updatedRoots))
	}

	// Reload from store
	catReloaded, err := New([]string{env.cliMovies}, env.browseBase, env.store)
	if err != nil || len(catReloaded.Roots()) != 2 {
		t.Fatalf("reloaded catalog roots mismatch: err=%v", err)
	}

	// Remove UI root
	seriesRootID := updatedRoots[1].ID
	if _, err := env.cat.RemoveUIRoot(updatedRoots[0].ID); err == nil {
		t.Errorf("expected removing CLI root to fail")
	}
	if _, err := env.cat.RemoveUIRoot("non-existent"); err == nil {
		t.Errorf("expected removing non-existent root to fail")
	}
	afterRemove, err := env.cat.RemoveUIRoot(seriesRootID)
	if err != nil || len(afterRemove) != 1 {
		t.Fatalf("RemoveUIRoot failed: %v", err)
	}
}

func TestCatalogBrowseSearchSubtitlesAndThumbnails(t *testing.T) {
	env := setupCatalogEnv(t)
	updatedRoots, err := env.cat.AddUIRoots([]string{"Series"})
	if err != nil {
		t.Fatal(err)
	}
	seriesRootID := updatedRoots[1].ID

	for _, sortOpt := range []string{"name_asc", "name_desc", "date_desc", "date_asc"} {
		browseSeries, bErr := env.cat.Browse(seriesRootID, "Temporada 1", sortOpt)
		if bErr != nil || len(browseSeries.Videos) != 2 {
			t.Fatalf("Browse (%s) failed: %v", sortOpt, bErr)
		}
		browseRoot, rErr := env.cat.Browse(seriesRootID, "", sortOpt)
		if rErr != nil || len(browseRoot.Folders) != 1 {
			t.Fatalf("Browse root (%s) failed: %v", sortOpt, rErr)
		}
	}

	sRoot := updatedRoots[1].SafeRoot()
	vttOut, err := ReadSubtitleAsWebVTT(sRoot, "Temporada 1/Episodio 01.pt-BR.srt")
	if err != nil || !strings.HasPrefix(string(vttOut), "WEBVTT\n\n") {
		t.Fatalf("ReadSubtitleAsWebVTT SRT failed: %v", err)
	}
	vttDirect, err := ReadSubtitleAsWebVTT(sRoot, "Temporada 1/Episodio 01.en.vtt")
	if err != nil || !strings.HasPrefix(string(vttDirect), "WEBVTT") {
		t.Fatalf("ReadSubtitleAsWebVTT VTT failed: %v", err)
	}

	if res, err := env.cat.Search("", "", "name_asc"); err != nil || res.Total != 0 {
		t.Errorf("empty search should return 0 results")
	}
	if res, err := env.cat.Search("episodio", seriesRootID, "date_asc"); err != nil || res.Total != 2 {
		t.Errorf("filtered search failed: err=%v total=%d", err, res.Total)
	}

	thumbBytes, thumbMime, err := env.cat.ResolveThumbnail(context.Background(), updatedRoots[0].ID, "Matrix.mp4", "", true)
	if err != nil || thumbMime != mimeImageJPEG || string(thumbBytes) != "fake-jpeg-bytes" {
		t.Errorf("ResolveThumbnail failed: err=%v mime=%s", err, thumbMime)
	}

	if !RequiresTranscodeByDefault("DragonBall.avi") || !RequiresTranscodeByDefault("ep1.rmvb") || RequiresTranscodeByDefault("movie.mp4") {
		t.Errorf("unexpected RequiresTranscodeByDefault classification")
	}
}
