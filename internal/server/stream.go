package server

import (
	"net/http"

	"github.com/cristiano/lan-video-player/internal/catalog"
	"github.com/cristiano/lan-video-player/internal/safefs"
)

// handleStream serves video files with full RFC 9110 HTTP Range Requests support (206 Partial Content)
// via Go's standard library http.ServeContent using strictly read-only file descriptors (FR-006, FR-008).
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_REQUEST", "Método HTTP não suportado.")
		return
	}

	rootID, relPath, ok := extractRootAndRelPath(r, "/api/v1/stream/")
	if !ok || rootID == "" || relPath == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Caminho de vídeo inválido.")
		return
	}

	root, err := s.cat.GetRoot(rootID)
	if err != nil {
		writeCatalogError(w, err)
		return
	}

	mimeType, isVideo := catalog.IsSupportedVideoFile(relPath)
	if !isVideo {
		writeError(w, http.StatusForbidden, "FORBIDDEN_PATH", "Formato de arquivo não permitido para streaming.")
		return
	}

	file, info, _, err := root.SafeRoot().OpenReadOnly(relPath)
	if err != nil {
		writeSafeFSError(w, err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-cache")

	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func writeSafeFSError(w http.ResponseWriter, err error) {
	switch {
	case isError(err, safefs.ErrForbiddenPath):
		writeError(w, http.StatusForbidden, "FORBIDDEN_PATH", "Acesso negado: caminho fora das pastas autorizadas.")
	case isError(err, safefs.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Arquivo ou pasta não encontrado.")
	case isError(err, safefs.ErrNotDirectory):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "O caminho informado não é um diretório.")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno ao acessar a biblioteca de mídia.")
	}
}
