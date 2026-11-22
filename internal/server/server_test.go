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

func mustWriteServerTestFile(t *testing.T, p string, data string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

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

	mustWriteServerTestFile(t, filepath.Join(moviesDir, "Demo Video.mp4"), "0123456789ABCDEFGHIJKLMNOPQRST")
	mustWriteServerTestFile(t, filepath.Join(moviesDir, "Demo Video.pt-BR.srt"), "1\n00:00:01,500 --> 00:00:03,000\nLegenda convertida em memória!\n")
	mustWriteServerTestFile(t, filepath.Join(coursesDir, "Aula 01.webm"), "webm-bytes-here")
	mustWriteServerTestFile(t, filepath.Join(coursesDir, "Aula 01.jpg"), "jpeg-thumb-data")

	outsideSecret := filepath.Join(outsideDir, "leak.mp4")
	mustWriteServerTestFile(t, outsideSecret, "top-secret")
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

func TestHTTPContractsRootsAndPicker(t *testing.T) {
	ts, _, _ := setupTestServer(t)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/v1/roots")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/roots status = %d", res.StatusCode)
	}

	var rootsPayload rootsResponse
	_ = json.NewDecoder(res.Body).Decode(&rootsPayload)
	moviesRootID := rootsPayload.Roots[0].ID

	pickerRes, err := http.Get(ts.URL + "/api/v1/fs/dirs")
	if err != nil {
		t.Fatal(err)
	}
	pickerRes.Body.Close()
	if pickerRes.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/fs/dirs status = %d", pickerRes.StatusCode)
	}

	pickerTraversal, err := http.Get(ts.URL + "/api/v1/fs/dirs?path=../../etc")
	if err != nil {
		t.Fatal(err)
	}
	pickerTraversal.Body.Close()
	if pickerTraversal.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 on picker traversal, got %d", pickerTraversal.StatusCode)
	}

	postRes, err := http.Post(ts.URL+"/api/v1/roots", "application/json", bytes.NewBufferString(`{"paths":["Cursos/Go"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var updatedRoots rootsResponse
	_ = json.NewDecoder(postRes.Body).Decode(&updatedRoots)
	postRes.Body.Close()
	coursesRootID := updatedRoots.Roots[1].ID

	browseRes, err := http.Get(ts.URL + "/api/v1/browse?root=" + moviesRootID)
	if err != nil {
		t.Fatal(err)
	}
	browseRes.Body.Close()
	if browseRes.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/browse status = %d", browseRes.StatusCode)
	}

	travRes, err := http.Get(ts.URL + "/api/v1/browse?root=" + moviesRootID + "&path=../secret_outside")
	if err != nil {
		t.Fatal(err)
	}
	travRes.Body.Close()
	if travRes.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for browse traversal, got %d", travRes.StatusCode)
	}

	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/roots/"+coursesRootID, nil)
	delRes, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	delRes.Body.Close()
	if delRes.StatusCode != http.StatusOK {
		t.Errorf("expected 200 on DELETE UI root, got %d", delRes.StatusCode)
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

	req, _ := http.NewRequest(http.MethodGet, streamURL, nil)
	req.Header.Set("Range", "bytes=10-19")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent || string(body) != "ABCDEFGHIJ" {
		t.Fatalf("expected 206 'ABCDEFGHIJ', got %d %q", resp.StatusCode, string(body))
	}

	req416, _ := http.NewRequest(http.MethodGet, streamURL, nil)
	req416.Header.Set("Range", "bytes=100-200")
	resp416, err := http.DefaultClient.Do(req416)
	if err != nil {
		t.Fatal(err)
	}
	resp416.Body.Close()
	if resp416.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("expected 416, got %d", resp416.StatusCode)
	}

	respEvil, err := http.Get(ts.URL + "/api/v1/stream/" + rootID + "/evil_link.mp4")
	if err != nil {
		t.Fatal(err)
	}
	respEvil.Body.Close()
	if respEvil.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for external symlink stream, got %d", respEvil.StatusCode)
	}
}

func TestHTTPSearchThumbnailsAndSubtitles(t *testing.T) {
	ts, _, moviesDir := setupTestServer(t)
	defer ts.Close()

	_, _ = http.Post(ts.URL+"/api/v1/roots", "application/json", bytes.NewBufferString(`{"paths":["Cursos/Go"]}`))

	sRes, err := http.Get(ts.URL + "/api/v1/search?q=aula")
	if err != nil {
		t.Fatal(err)
	}
	var searchOut catalog.SearchResult
	_ = json.NewDecoder(sRes.Body).Decode(&searchOut)
	sRes.Body.Close()

	thumbRes, err := http.Get(ts.URL + searchOut.Videos[0].ThumbnailURL)
	if err != nil {
		t.Fatal(err)
	}
	thumbData, _ := io.ReadAll(thumbRes.Body)
	thumbRes.Body.Close()
	if thumbRes.StatusCode != http.StatusOK || string(thumbData) != "jpeg-thumb-data" {
		t.Errorf("expected 200 sidecar thumbnail, got %d", thumbRes.StatusCode)
	}

	sDemoRes, err := http.Get(ts.URL + "/api/v1/search?q=demo")
	if err != nil {
		t.Fatal(err)
	}
	var demoOut catalog.SearchResult
	_ = json.NewDecoder(sDemoRes.Body).Decode(&demoOut)
	sDemoRes.Body.Close()

	noThumbRes, err := http.Get(ts.URL + demoOut.Videos[0].ThumbnailURL)
	if err != nil {
		t.Fatal(err)
	}
	noThumbRes.Body.Close()
	if noThumbRes.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204 No Content, got %d", noThumbRes.StatusCode)
	}

	subRes, err := http.Get(ts.URL + demoOut.Videos[0].Subtitles[0].SrcURL)
	if err != nil {
		t.Fatal(err)
	}
	subBody, _ := io.ReadAll(subRes.Body)
	subRes.Body.Close()
	if subRes.StatusCode != http.StatusOK || !strings.HasPrefix(string(subBody), "WEBVTT\n\n") {
		t.Errorf("expected 200 WEBVTT, got %d", subRes.StatusCode)
	}

	// Test subtitle offset + delay query params
	subShiftRes, err := http.Get(ts.URL + demoOut.Videos[0].Subtitles[0].SrcURL + "?offset=1&delay=3.5")
	if err != nil {
		t.Fatal(err)
	}
	subShiftBody, _ := io.ReadAll(subShiftRes.Body)
	subShiftRes.Body.Close()
	if subShiftRes.StatusCode != http.StatusOK || !strings.Contains(string(subShiftBody), "00:00:04.000 --> 00:00:05.500") {
		t.Errorf("expected shifted WEBVTT timestamps (+2.5s), got status=%d body=%q", subShiftRes.StatusCode, string(subShiftBody))
	}

	entries, _ := os.ReadDir(moviesDir)
	if len(entries) != 3 {
		t.Errorf("expected moviesDir untouched with 3 entries, got %d", len(entries))
	}
}

func TestHTTPLicenseProbeAndErrorStatuses(t *testing.T) {
	ts, _, _ := setupTestServer(t)
	defer ts.Close()

	licRes, err := http.Get(ts.URL + "/LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	licBody, _ := io.ReadAll(licRes.Body)
	licRes.Body.Close()
	if licRes.StatusCode != http.StatusOK || !strings.Contains(string(licBody), "GNU AFFERO") {
		t.Errorf("unexpected /LICENSE response")
	}

	sDemoRes, _ := http.Get(ts.URL + "/api/v1/search?q=demo")
	var demoOut catalog.SearchResult
	_ = json.NewDecoder(sDemoRes.Body).Decode(&demoOut)
	sDemoRes.Body.Close()

	probeURL := ts.URL + strings.Replace(demoOut.Videos[0].StreamURL, "/api/v1/stream/", "/api/v1/probe/", 1)
	probeRes, err := http.Get(probeURL)
	if err != nil {
		t.Fatal(err)
	}
	probeRes.Body.Close()
	if probeRes.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK from /api/v1/probe, got %d", probeRes.StatusCode)
	}

	for _, endpoint := range []string{"/api/v1/browse", "/api/v1/search", "/api/v1/fs/dirs", "/api/v1/probe/r/v.mp4", "/api/v1/subtitles/r/s.srt", "/api/v1/thumbnails/r/v.mp4", "/api/v1/stream/r/v.mp4", "/api/v1/roots/r"} {
		postResp, pErr := http.Post(ts.URL+endpoint, "application/json", bytes.NewBufferString(`{}`))
		if pErr != nil {
			t.Fatal(pErr)
		}
		postResp.Body.Close()
		if postResp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for POST %s, got %d", endpoint, postResp.StatusCode)
		}
	}
}
