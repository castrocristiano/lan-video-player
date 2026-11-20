package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"github.com/cristiano/lan-video-player/internal/catalog"
	"github.com/cristiano/lan-video-player/internal/config"
	"github.com/cristiano/lan-video-player/internal/safefs"
	"github.com/cristiano/lan-video-player/web"
)

const (
	offlineCSP          = "default-src 'self'; media-src 'self' blob:; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; font-src 'self' data:; object-src 'none'; frame-ancestors 'none'"
	errMethodNotAllowed = "Método HTTP não suportado."
	headerContentType   = "Content-Type"
)

// Server implements the HTTP handler and API endpoints for lan-video-player.
type Server struct {
	cat      *catalog.Catalog
	cacheDir string
	noThumbs bool
	mux      *http.ServeMux
}

// New creates and configures a new HTTP Server instance.
func New(cat *catalog.Catalog, cacheDir string, noThumbs bool) (*Server, error) {
	s := &Server{
		cat:      cat,
		cacheDir: cacheDir,
		noThumbs: noThumbs,
		mux:      http.NewServeMux(),
	}

	if err := s.registerRoutes(); err != nil {
		return nil, err
	}
	return s, nil
}

// ServeHTTP applies offline security headers (CSP) and dispatches the request.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", offlineCSP)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	s.mux.ServeHTTP(w, r)
}

func (s *Server) registerRoutes() error {
	s.mux.HandleFunc("/api/v1/roots", s.handleRoots)
	s.mux.HandleFunc("/api/v1/roots/", s.handleRootByID)
	s.mux.HandleFunc("/api/v1/fs/dirs", s.handleBrowseBaseDirs)
	s.mux.HandleFunc("/api/v1/browse", s.handleBrowse)
	s.mux.HandleFunc("/api/v1/search", s.handleSearch)
	s.mux.HandleFunc("/api/v1/stream/", s.handleStream)
	s.mux.HandleFunc("/api/v1/probe/", s.handleProbe)
	s.mux.HandleFunc("/api/v1/subtitles/", s.handleSubtitles)
	s.mux.HandleFunc("/api/v1/thumbnails/", s.handleThumbnails)
	s.mux.HandleFunc("/LICENSE", s.handleLicense)

	staticFS, err := web.StaticFS()
	if err != nil {
		return err
	}
	fileServer := http.FileServer(http.FS(staticFS))
	s.mux.Handle("/", fileServer)
	return nil
}

// rootsResponse matches contracts/http-api.md 2.1 GET /api/v1/roots.
type rootsResponse struct {
	Roots      []catalog.MediaRoot `json:"roots"`
	BrowseBase browseBaseMeta      `json:"browseBase"`
	Meta       appMeta             `json:"meta"`
}

type browseBaseMeta struct {
	Enabled bool   `json:"enabled"`
	Label   string `json:"label"`
}

type appMeta struct {
	AppName   string `json:"appName"`
	Version   string `json:"version"`
	License   string `json:"license"`
	SourceURL string `json:"sourceUrl"`
}

type addRootsRequest struct {
	Paths []string `json:"paths"`
}

func (s *Server) buildRootsPayload(roots []catalog.MediaRoot) rootsResponse {
	if roots == nil {
		roots = []catalog.MediaRoot{}
	}
	return rootsResponse{
		Roots: roots,
		BrowseBase: browseBaseMeta{
			Enabled: s.cat.BrowseBaseEnabled(),
			Label:   s.cat.BrowseBaseLabel(),
		},
		Meta: appMeta{
			AppName:   "lan-video-player",
			Version:   config.AppVersion,
			License:   "GNU AGPL v3.0",
			SourceURL: "/LICENSE",
		},
	}
}

// handleRoots handles GET /api/v1/roots and POST /api/v1/roots.
func (s *Server) handleRoots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.buildRootsPayload(s.cat.Roots()))
	case http.MethodPost:
		var req addRootsRequest
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
		if err := dec.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "JSON inválido na requisição.")
			return
		}
		if len(req.Paths) == 0 {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Selecione pelo menos uma pasta para adicionar à biblioteca.")
			return
		}
		updated, err := s.cat.AddUIRoots(req.Paths)
		if err != nil {
			writeCatalogError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, s.buildRootsPayload(updated))
	default:
		writeError(w, http.StatusMethodNotAllowed, "INVALID_REQUEST", errMethodNotAllowed)
	}
}

// handleRootByID handles DELETE /api/v1/roots/{rootId}.
func (s *Server) handleRootByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_REQUEST", errMethodNotAllowed)
		return
	}
	rootID := strings.TrimPrefix(r.URL.Path, "/api/v1/roots/")
	rootID = strings.Trim(rootID, "/")
	if decoded, err := url.PathUnescape(rootID); err == nil {
		rootID = decoded
	}
	if rootID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Identificador de raiz ausente.")
		return
	}
	updated, err := s.cat.RemoveUIRoot(rootID)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.buildRootsPayload(updated))
}

// handleBrowseBaseDirs handles GET /api/v1/fs/dirs?path=... for the visual folder picker (FR-018).
func (s *Server) handleBrowseBaseDirs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_REQUEST", errMethodNotAllowed)
		return
	}
	relPath := r.URL.Query().Get("path")
	res, err := s.cat.ListBrowseBaseDirs(relPath)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleBrowse handles GET /api/v1/browse?root=...&path=...&sort=...
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_REQUEST", errMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	rootID := q.Get("root")
	relPath := q.Get("path")
	sortBy := q.Get("sort")

	res, err := s.cat.Browse(rootID, relPath, sortBy)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleSearch handles GET /api/v1/search?q=...&root=...&sort=...
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_REQUEST", errMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	query := q.Get("q")
	rootID := q.Get("root")
	sortBy := q.Get("sort")

	res, err := s.cat.Search(query, rootID, sortBy)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleProbe handles GET /api/v1/probe/{rootId}/{path...} returning video duration and transcode info.
func (s *Server) handleProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_REQUEST", errMethodNotAllowed)
		return
	}
	rootID, relPath, ok := extractRootAndRelPath(r, "/api/v1/probe/")
	if !ok || rootID == "" || relPath == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Caminho de vídeo inválido.")
		return
	}
	res, err := s.cat.ProbeVideo(r.Context(), rootID, relPath)
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleSubtitles handles GET /api/v1/subtitles/{rootId}/{path...} serving WebVTT (FR-014).
func (s *Server) handleSubtitles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_REQUEST", errMethodNotAllowed)
		return
	}
	rootID, relPath, ok := extractRootAndRelPath(r, "/api/v1/subtitles/")
	if !ok || rootID == "" || relPath == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Caminho de legenda inválido.")
		return
	}

	root, err := s.cat.GetRoot(rootID)
	if err != nil {
		writeCatalogError(w, err)
		return
	}

	vttData, err := catalog.ReadSubtitleAsWebVTT(root.SafeRoot(), relPath)
	if err != nil {
		writeSafeFSError(w, err)
		return
	}

	w.Header().Set(headerContentType, "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(vttData)
	}
}

// handleThumbnails handles GET /api/v1/thumbnails/{rootId}/{path...} (FR-005, FR-008).
func (s *Server) handleThumbnails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_REQUEST", errMethodNotAllowed)
		return
	}
	rootID, relPath, ok := extractRootAndRelPath(r, "/api/v1/thumbnails/")
	if !ok || rootID == "" || relPath == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Caminho de miniatura inválido.")
		return
	}

	data, contentType, err := s.cat.ResolveThumbnail(r.Context(), rootID, relPath, s.cacheDir, s.noThumbs)
	if err != nil {
		if errors.Is(err, catalog.ErrNoThumbnail) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeCatalogError(w, err)
		return
	}

	w.Header().Set(headerContentType, contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

// handleLicense serves the full GNU AGPL v3.0 license text (FR-016).
func (s *Server) handleLicense(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(headerContentType, "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(web.LicenseBytes())
}

// extractRootAndRelPath extracts {rootId} and {path...} from raw request paths,
// preserving and decoding URL-encoded segments safely.
func extractRootAndRelPath(r *http.Request, prefix string) (rootID string, relPath string, ok bool) {
	rawPath := r.URL.EscapedPath()
	if rawPath == "" {
		rawPath = r.URL.Path
	}
	if !strings.HasPrefix(rawPath, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(rawPath, prefix)
	slashIdx := strings.IndexByte(rest, '/')
	if slashIdx <= 0 || slashIdx == len(rest)-1 {
		return "", "", false
	}

	rawRootID := rest[:slashIdx]
	rawRel := rest[slashIdx+1:]

	decodedRoot, err := url.PathUnescape(rawRootID)
	if err != nil {
		return "", "", false
	}

	rawSegments := strings.Split(rawRel, "/")
	decodedSegments := make([]string, 0, len(rawSegments))
	for _, seg := range rawSegments {
		decSeg, uErr := url.PathUnescape(seg)
		if uErr != nil {
			return "", "", false
		}
		// Reject raw or encoded "/" inside a single path segment
		if strings.Contains(decSeg, "/") || strings.Contains(decSeg, "\\") {
			return "", "", false
		}
		decodedSegments = append(decodedSegments, decSeg)
	}

	return decodedRoot, strings.Join(decodedSegments, "/"), true
}

func writeCatalogError(w http.ResponseWriter, err error) {
	switch {
	case isError(err, safefs.ErrForbiddenPath):
		writeError(w, http.StatusForbidden, "FORBIDDEN_PATH", "Acesso negado: caminho fora das pastas autorizadas.")
	case isError(err, safefs.ErrNotFound), isError(err, fs.ErrNotExist):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Pasta ou arquivo não encontrado.")
	case isError(err, catalog.ErrRootNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Pasta raiz da biblioteca não encontrada.")
	case isError(err, catalog.ErrRootNotRemovable):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Pastas fixadas via linha de comando não podem ser removidas pela interface.")
	case isError(err, catalog.ErrBrowseBaseDisabled):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "O diretório-base de seleção de pastas não está habilitado.")
	case isError(err, safefs.ErrNotDirectory):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "O caminho selecionado não é uma pasta válida.")
	default:
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
	}
}

func isError(err, target error) bool {
	return errors.Is(err, target)
}

type apiErrorEnvelope struct {
	Error apiErrorDetail `json:"error"`
}

type apiErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, apiErrorEnvelope{
		Error: apiErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set(headerContentType, "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}
