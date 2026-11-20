package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristiano/lan-video-player/internal/catalog"
	"github.com/cristiano/lan-video-player/internal/config"
)

func setupTestServer(t *testing.T) (*httptest.Server, string, string) {
	t.Helper()
	tmp := t.TempDir()
	browseBase := filepath.Join(tmp, "media")
	moviesDir := filepath.Join(browseBase, "Filmes")
	coursesDir := filepath.Join(browseBase, "Cursos", "Go")
	outsideDir := filepath.Join(tmp, "secret_outside")

	for _, d := range []string{moviesDir, coursesDir, outsideDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Create 30-byte video file for HTTP Range tests
	videoContent := []byte("0123456789ABCDEFGHIJKLMNOPQRST")
	videoPath := filepath.Join(moviesDir, "Demo Video.mp4")
	if err := os.WriteFile(videoPath, videoContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create sidecar SRT subtitle
	srtPath := filepath.Join(moviesDir, "Demo Video.pt-BR.srt")
	srtData := "1\n00:00:01,500 --> 00:00:03,000\nLegenda convertida em memória!\n"
	if err := os.WriteFile(srtPath, []byte(srtData), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create video inside coursesDir (to be added via POST /api/v1/roots)
	courseVid := filepath.Join(coursesDir, "Aula 01.webm")
	if err := os.WriteFile(courseVid, []byte("webm-bytes-here"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Sidecar thumbnail for Aula 01
	if err := os.WriteFile(filepath.Join(coursesDir, "Aula 01.jpg"), []byte("jpeg-thumb-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create external secret and symlink inside moviesDir pointing outside
	outsideSecret := filepath.Join(outsideDir, "leak.mp4")
	if err := os.WriteFile(outsideSecret, []byte("top-secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Symlink(outsideSecret, filepath.Join(moviesDir, "evil_link.mp4"))

	store := config.NewStore(filepath.Join(tmp, "config", "library.json"))
	cat, err := catalog.New([]string{moviesDir}, browseBase, store)
	if err != nil {
		t.Fatalf("catalog.New failed: %v", err)
	}

	srv, err := New(cat, filepath.Join(tmp, "cache"), true)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	return httptest.NewServer(srv), browseBase, moviesDir
}

func TestHTTPContractsRootsPickerAndBrowse(t *testing.T) {
	ts, _, _ := setupTestServer(t)
	defer ts.Close()

	// 1. GET /api/v1/roots
	res, err := http.Get(ts.URL + "/api/v1/roots")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/roots status = %d", res.StatusCode)
	}
	if csp := res.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("missing offline CSP header: %q", csp)
	}

	var rootsPayload rootsResponse
	if err := json.NewDecoder(res.Body).Decode(&rootsPayload); err != nil {
		t.Fatal(err)
	}
	if len(rootsPayload.Roots) != 1 || rootsPayload.Meta.License != "GNU AGPL v3.0" {
		t.Fatalf("unexpected roots payload: %+v", rootsPayload)
	}
	moviesRootID := rootsPayload.Roots[0].ID

	// 2. GET /api/v1/fs/dirs (Visual folder picker)
	pickerRes, err := http.Get(ts.URL + "/api/v1/fs/dirs")
	if err != nil {
		t.Fatal(err)
	}
	defer pickerRes.Body.Close()
	if pickerRes.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/fs/dirs status = %d", pickerRes.StatusCode)
	}

	// Path traversal in picker must return 403 Forbidden
	pickerTraversal, err := http.Get(ts.URL + "/api/v1/fs/dirs?path=../../etc")
	if err != nil {
		t.Fatal(err)
	}
	pickerTraversal.Body.Close()
	if pickerTraversal.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden on picker path traversal, got %d", pickerTraversal.StatusCode)
	}

	// 3. POST /api/v1/roots to add Cursos/Go folder dynamically
	postBody := bytes.NewBufferString(`{"paths":["Cursos/Go"]}`)
	postRes, err := http.Post(ts.URL+"/api/v1/roots", "application/json", postBody)
	if err != nil {
		t.Fatal(err)
	}
	defer postRes.Body.Close()
	if postRes.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/v1/roots expected 200 OK, got %d", postRes.StatusCode)
	}
	var updatedRoots rootsResponse
	if err := json.NewDecoder(postRes.Body).Decode(&updatedRoots); err != nil {
		t.Fatal(err)
	}
	if len(updatedRoots.Roots) != 2 {
		t.Fatalf("expected 2 roots after POST, got %d", len(updatedRoots.Roots))
	}
	coursesRootID := updatedRoots.Roots[1].ID

	// 4. GET /api/v1/browse on moviesRootID
	browseRes, err := http.Get(ts.URL + "/api/v1/browse?root=" + moviesRootID)
	if err != nil {
		t.Fatal(err)
	}
	defer browseRes.Body.Close()
	if browseRes.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/browse status = %d", browseRes.StatusCode)
	}
	var bResult catalog.BrowseResult
	if err := json.NewDecoder(browseRes.Body).Decode(&bResult); err != nil {
		t.Fatal(err)
	}
	if len(bResult.Videos) != 1 || bResult.Videos[0].Name != "Demo Video.mp4" {
		t.Fatalf("expected only Demo Video.mp4 (evil_link.mp4 filtered out), got %+v", bResult.Videos)
	}

	// 5. GET /api/v1/browse with path traversal must return 403 FORBIDDEN_PATH
	travRes, err := http.Get(ts.URL + "/api/v1/browse?root=" + moviesRootID + "&path=../secret_outside")
	if err != nil {
		t.Fatal(err)
	}
	defer travRes.Body.Close()
	if travRes.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for browse path traversal, got %d", travRes.StatusCode)
	}

	// 6. DELETE /api/v1/roots/{coursesRootID}
	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/roots/"+coursesRootID, nil)
	delRes, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	delRes.Body.Close()
	if delRes.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK on DELETE UI root, got %d", delRes.StatusCode)
	}
}

func TestHTTPRangeStreamingAndSecurity(t *testing.T) {
	ts, _, _ := setupTestServer(t)
	defer ts.Close()

	rootsRes, err := http.Get(ts.URL + "/api/v1/roots")
	if err != nil {
		t.Fatal(err)
	}
	var rp rootsResponse
	_ = json.NewDecoder(rootsRes.Body).Decode(&rp)
	rootsRes.Body.Close()
	rootID := rp.Roots[0].ID

	streamURL := ts.URL + "/api/v1/stream/" + rootID + "/Demo%20Video.mp4"

	// 1. Partial Range Request (bytes=10-19) -> 206 Partial Content
	req, _ := http.NewRequest(http.MethodGet, streamURL, nil)
	req.Header.Set("Range", "bytes=10-19")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content, got %d", resp.StatusCode)
	}
	if string(body) != "ABCDEFGHIJ" {
		t.Errorf("expected bytes 10-19 'ABCDEFGHIJ', got %q", string(body))
	}
	if cr := resp.Header.Get("Content-Range"); cr != "bytes 10-19/30" {
		t.Errorf("expected Content-Range 'bytes 10-19/30', got %q", cr)
	}

	// 2. Out-of-bounds Range Request -> 416 Range Not Satisfiable
	req416, _ := http.NewRequest(http.MethodGet, streamURL, nil)
	req416.Header.Set("Range", "bytes=100-200")
	resp416, err := http.DefaultClient.Do(req416)
	if err != nil {
		t.Fatal(err)
	}
	resp416.Body.Close()
	if resp416.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("expected 416 Range Not Satisfiable, got %d", resp416.StatusCode)
	}

	// 3. Symlink escape attempt via stream endpoint -> 403 Forbidden
	evilURL := ts.URL + "/api/v1/stream/" + rootID + "/evil_link.mp4"
	respEvil, err := http.Get(evilURL)
	if err != nil {
		t.Fatal(err)
	}
	respEvil.Body.Close()
	if respEvil.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for external symlink stream, got %d", respEvil.StatusCode)
	}
}

func TestHTTPSearchThumbnailsAndSubtitles(t *testing.T) {
	ts, _, moviesDir := setupTestServer(t)
	defer ts.Close()

	// Add Cursos/Go root so both roots are searchable
	_, _ = http.Post(ts.URL+"/api/v1/roots", "application/json", bytes.NewBufferString(`{"paths":["Cursos/Go"]}`))

	// 1. GET /api/v1/search?q=aula
	sRes, err := http.Get(ts.URL + "/api/v1/search?q=aula")
	if err != nil {
		t.Fatal(err)
	}
	var searchOut catalog.SearchResult
	_ = json.NewDecoder(sRes.Body).Decode(&searchOut)
	sRes.Body.Close()
	if searchOut.Total != 1 || searchOut.Videos[0].Name != "Aula 01.webm" {
		t.Fatalf("expected search for 'aula' to find 'Aula 01.webm', got %+v", searchOut)
	}

	// 2. GET /api/v1/thumbnails for Aula 01.webm (has sidecar Aula 01.jpg -> 200 OK)
	thumbRes, err := http.Get(ts.URL + searchOut.Videos[0].ThumbnailURL)
	if err != nil {
		t.Fatal(err)
	}
	thumbData, _ := io.ReadAll(thumbRes.Body)
	thumbRes.Body.Close()
	if thumbRes.StatusCode != http.StatusOK || string(thumbData) != "jpeg-thumb-data" {
		t.Errorf("expected 200 OK sidecar thumbnail, got status=%d body=%q", thumbRes.StatusCode, string(thumbData))
	}

	// 3. GET /api/v1/search?q=demo -> test 204 No Content thumbnail fallback & SRT subtitle conversion
	sDemoRes, err := http.Get(ts.URL + "/api/v1/search?q=demo")
	if err != nil {
		t.Fatal(err)
	}
	var demoOut catalog.SearchResult
	_ = json.NewDecoder(sDemoRes.Body).Decode(&demoOut)
	sDemoRes.Body.Close()
	if demoOut.Total != 1 {
		t.Fatalf("expected Demo Video.mp4 in search, got %d", demoOut.Total)
	}

	noThumbRes, err := http.Get(ts.URL + demoOut.Videos[0].ThumbnailURL)
	if err != nil {
		t.Fatal(err)
	}
	noThumbRes.Body.Close()
	if noThumbRes.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204 No Content when no thumbnail exists and noThumbs=true, got %d", noThumbRes.StatusCode)
	}

	// 4. GET /api/v1/subtitles for Demo Video.pt-BR.srt -> must serve text/vtt converted in memory
	if len(demoOut.Videos[0].Subtitles) != 1 {
		t.Fatalf("expected 1 subtitle attached to Demo Video.mp4, got %d", len(demoOut.Videos[0].Subtitles))
	}
	subURL := ts.URL + demoOut.Videos[0].Subtitles[0].SrcURL
	subRes, err := http.Get(subURL)
	if err != nil {
		t.Fatal(err)
	}
	subBody, _ := io.ReadAll(subRes.Body)
	subRes.Body.Close()
	if subRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from subtitle endpoint, got %d", subRes.StatusCode)
	}
	if ct := subRes.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/vtt") {
		t.Errorf("expected text/vtt Content-Type, got %q", ct)
	}
	if !strings.HasPrefix(string(subBody), "WEBVTT\n\n") || !strings.Contains(string(subBody), "00:00:01.500 --> 00:00:03.000") {
		t.Errorf("unexpected VTT body: %q", string(subBody))
	}

	// 5. Verify zero files were written inside moviesDir (FR-008)
	entries, err := os.ReadDir(moviesDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 { // Demo Video.mp4, Demo Video.pt-BR.srt, evil_link.mp4
		t.Errorf("expected moviesDir to remain untouched with 3 entries, got %d", len(entries))
	}
}
