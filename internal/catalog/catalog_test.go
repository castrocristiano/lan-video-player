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

func TestCatalogBrowsePickerSearchSubtitlesAndThumbnails(t *testing.T) {
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

	// Create sample videos, non-video files, hidden files, sidecar subtitles, and sidecar thumbnail
	v1 := filepath.Join(cliMovies, "Matrix.mp4")
	if err := os.WriteFile(v1, []byte("video-matrix"), 0o644); err != nil {
		t.Fatal(err)
	}
	v1Thumb := filepath.Join(cliMovies, "Matrix.jpg")
	if err := os.WriteFile(v1Thumb, []byte("fake-jpeg-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	nonVideo := filepath.Join(cliMovies, "notes.txt")
	if err := os.WriteFile(nonVideo, []byte("ignore me"), 0o644); err != nil {
		t.Fatal(err)
	}
	hiddenVideo := filepath.Join(cliMovies, ".hidden.mp4")
	if err := os.WriteFile(hiddenVideo, []byte("ignore hidden"), 0o644); err != nil {
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
	// Make ep2 newer than ep1 for date sorting test
	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-10 * time.Minute)
	_ = os.Chtimes(ep1, oldTime, oldTime)
	_ = os.Chtimes(ep2, newTime, newTime)

	// Sidecar SRT and VTT subtitles for Episodio 01
	srtContent := "1\r\n00:00:01,250 --> 00:00:04,500\r\nOlá Rede Local!\r\n"
	if err := os.WriteFile(filepath.Join(uiSeries, "Episodio 01.pt-BR.srt"), []byte(srtContent), 0o644); err != nil {
		t.Fatal(err)
	}
	vttContent := "WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nHello LAN!\n"
	if err := os.WriteFile(filepath.Join(uiSeries, "Episodio 01.en.vtt"), []byte(vttContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgFile := filepath.Join(tmp, "config", "library.json")
	store := config.NewStore(cfgFile)

	cat, err := New([]string{cliMovies}, browseBase, store)
	if err != nil {
		t.Fatalf("New catalog failed: %v", err)
	}

	// 1. Verify initial CLI root
	roots := cat.Roots()
	if len(roots) != 1 {
		t.Fatalf("expected 1 initial root, got %d", len(roots))
	}
	if roots[0].Source != "cli" || roots[0].Removable {
		t.Errorf("CLI root should have source=cli and removable=false, got %+v", roots[0])
	}

	// 2. Test visual folder picker inside --browse-base (T011)
	pickerRes, err := cat.ListBrowseBaseDirs("")
	if err != nil {
		t.Fatalf("ListBrowseBaseDirs failed: %v", err)
	}
	if len(pickerRes.Directories) != 2 {
		t.Fatalf("expected 2 directories in browseBase (Filmes, Series), got %d", len(pickerRes.Directories))
	}
	for _, d := range pickerRes.Directories {
		if d.Name == "Filmes" && !d.AlreadyAdded {
			t.Errorf("expected Filmes to be marked AlreadyAdded=true")
		}
		if d.Name == "Series" && !d.HasSubdirs {
			t.Errorf("expected Series to have HasSubdirs=true")
		}
	}

	// 3. Add "Series" folder via AddUIRoots and verify persistence in library.json (FR-018, FR-019)
	updatedRoots, err := cat.AddUIRoots([]string{"Series"})
	if err != nil {
		t.Fatalf("AddUIRoots failed: %v", err)
	}
	if len(updatedRoots) != 2 {
		t.Fatalf("expected 2 roots after AddUIRoots, got %d", len(updatedRoots))
	}
	seriesRootID := updatedRoots[1].ID
	if updatedRoots[1].Source != "ui" || !updatedRoots[1].Removable {
		t.Errorf("expected UI root to have source=ui and removable=true, got %+v", updatedRoots[1])
	}

	// Reload a brand-new Catalog instance to confirm persistence across restarts
	catReloaded, err := New([]string{cliMovies}, browseBase, store)
	if err != nil {
		t.Fatalf("New reloaded catalog failed: %v", err)
	}
	if len(catReloaded.Roots()) != 2 {
		t.Fatalf("expected reloaded catalog to preserve 2 roots from library.json, got %d", len(catReloaded.Roots()))
	}

	// 4. Test Browse & Breadcrumbs & Subtitles Discovery (T011, T029)
	browseSeries, err := cat.Browse(seriesRootID, "Temporada 1", "name_asc")
	if err != nil {
		t.Fatalf("Browse Temporada 1 failed: %v", err)
	}
	if len(browseSeries.Breadcrumbs) != 2 {
		t.Errorf("expected 2 breadcrumbs, got %+v", browseSeries.Breadcrumbs)
	}
	if len(browseSeries.Videos) != 2 {
		t.Fatalf("expected 2 videos in Temporada 1, got %d", len(browseSeries.Videos))
	}
	if len(browseSeries.Videos[0].Subtitles) != 2 {
		t.Fatalf("expected 2 discovered subtitles for Episodio 01, got %+v", browseSeries.Videos[0].Subtitles)
	}

	// 5. Test in-memory SRT -> WebVTT conversion (T029)
	sRoot := updatedRoots[1].SafeRoot()
	vttOut, err := ReadSubtitleAsWebVTT(sRoot, "Temporada 1/Episodio 01.pt-BR.srt")
	if err != nil {
		t.Fatalf("ReadSubtitleAsWebVTT failed: %v", err)
	}
	vttStr := string(vttOut)
	if !strings.HasPrefix(vttStr, "WEBVTT\n\n") || !strings.Contains(vttStr, "00:00:01.250 --> 00:00:04.500") {
		t.Errorf("unexpected WebVTT converted output:\n%s", vttStr)
	}

	// 6. Test Search & Sorting (T023)
	searchRes, err := cat.Search("episodio", "", "name_desc")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if searchRes.Total != 2 {
		t.Fatalf("expected 2 search matches for 'episodio', got %d", searchRes.Total)
	}
	if searchRes.Videos[0].Name != "Episodio 02.webm" {
		t.Errorf("expected name_desc sort to put Episodio 02.webm first, got %q", searchRes.Videos[0].Name)
	}

	searchByDate, err := cat.Search("episodio", "", "date_desc")
	if err != nil {
		t.Fatalf("Search date_desc failed: %v", err)
	}
	if searchByDate.Videos[0].Name != "Episodio 02.webm" {
		t.Errorf("expected date_desc sort to put newer Episodio 02.webm first, got %q", searchByDate.Videos[0].Name)
	}

	// 7. Test Read-Only Sidecar Thumbnail Resolution (T023)
	thumbBytes, thumbMime, err := cat.ResolveThumbnail(context.Background(), updatedRoots[0].ID, "Matrix.mp4", "", true)
	if err != nil {
		t.Fatalf("ResolveThumbnail for sidecar Matrix.jpg failed: %v", err)
	}
	if thumbMime != "image/jpeg" || string(thumbBytes) != "fake-jpeg-bytes" {
		t.Errorf("unexpected thumbnail result: mime=%s, len=%d", thumbMime, len(thumbBytes))
	}

	// 8. Test unlinking UI root without deleting files on disk
	if _, err := cat.RemoveUIRoot(updatedRoots[0].ID); err == nil {
		t.Errorf("expected removing CLI root to fail with ErrRootNotRemovable")
	}
	afterRemove, err := cat.RemoveUIRoot(seriesRootID)
	if err != nil {
		t.Fatalf("RemoveUIRoot failed: %v", err)
	}
	if len(afterRemove) != 1 {
		t.Fatalf("expected 1 root remaining after RemoveUIRoot, got %d", len(afterRemove))
	}
	if _, err := os.Stat(ep1); err != nil {
		t.Errorf("RemoveUIRoot must never delete files from disk, but ep1 is missing: %v", err)
	}
}
