package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cristiano/lan-video-player/internal/catalog"
	"github.com/cristiano/lan-video-player/internal/safefs"
)

// handleStream serves video files with full RFC 9110 HTTP Range Requests support (206 Partial Content)
// via Go's standard library http.ServeContent using strictly read-only file descriptors (FR-006, FR-008).
// For legacy containers unsupported natively by web browsers (.avi, .rmvb, .wmv, .flv) or when
// ?transcode=1 is requested, it transparently transcodes on-the-fly to fragmented MP4 (H.264/AAC)
// in memory via ffmpeg when available, without writing to the media directories.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "INVALID_REQUEST", errMethodNotAllowed)
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

	q := r.URL.Query()
	wantTranscode := q.Get("transcode") == "1" || (q.Get("direct") != "1" && catalog.RequiresTranscodeByDefault(relPath))

	if wantTranscode && catalog.HasFFmpeg() {
		targetReal, _, rErr := root.SafeRoot().Resolve(relPath)
		if rErr != nil {
			writeSafeFSError(w, rErr)
			return
		}
		// Verify file exists and is a regular readable file
		f, _, _, oErr := root.SafeRoot().OpenReadOnly(relPath)
		if oErr != nil {
			writeSafeFSError(w, oErr)
			return
		}
		_ = f.Close()

		s.streamTranscodedMP4(w, r, targetReal, q.Get("start"))
		return
	}

	file, info, _, err := root.SafeRoot().OpenReadOnly(relPath)
	if err != nil {
		writeSafeFSError(w, err)
		return
	}
	defer file.Close()

	w.Header().Set(headerContentType, mimeType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-cache")

	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func (s *Server) streamTranscodedMP4(w http.ResponseWriter, r *http.Request, targetRealPath string, startSecStr string) {
	w.Header().Set(headerContentType, "video/mp4")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Transcoded-Stream", "ffmpeg-fmp4")

	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
	}
	if startSecStr != "" {
		if sec, err := strconv.ParseFloat(startSecStr, 64); err == nil && sec > 0 {
			args = append(args, "-ss", fmt.Sprintf("%.2f", sec))
		}
	}

	args = append(args,
		"-i", targetRealPath,
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ac", "2",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4",
		"pipe:1",
	)

	cmd, ok := catalog.NewFFmpegCommand(r.Context(), args...)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "FFmpeg indisponível para conversão em tempo real.")
		return
	}

	cmd.Stdout = w
	w.WriteHeader(http.StatusOK)
	_ = cmd.Run()
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
