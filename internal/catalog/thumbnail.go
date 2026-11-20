package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cristiano/lan-video-player/internal/safefs"
)

// ErrNoThumbnail indicates no sidecar thumbnail exists and server-side extraction is unavailable.
var ErrNoThumbnail = errors.New("thumbnail unavailable on server")

const mimeImageJPEG = "image/jpeg"

var sidecarImageExtensions = []struct {
	ext  string
	mime string
}{
	{".jpg", mimeImageJPEG},
	{".jpeg", mimeImageJPEG},
	{".png", "image/png"},
	{".webp", "image/webp"},
}

// ResolveThumbnail attempts to obtain a video thumbnail without ever writing to the media root:
// 1. Read-only lookup for an existing sidecar image (<video_base>.jpg/.png/.webp or poster.jpg).
// 2. If ffmpeg is available in PATH and !noThumbs, extracts 1 JPEG frame into cacheDir (outside media roots).
// 3. Otherwise returns ErrNoThumbnail so the caller can return HTTP 204 No Content.
func (c *Catalog) ResolveThumbnail(ctx context.Context, rootID string, videoRelPath string, cacheDir string, noThumbs bool) ([]byte, string, error) {
	root, err := c.GetRoot(rootID)
	if err != nil {
		return nil, "", err
	}

	// Validate that the target video exists and is a supported video file inside root
	videoRealPath, normRel, err := root.safeRoot.Resolve(videoRelPath)
	if err != nil {
		return nil, "", err
	}
	if _, ok := IsSupportedVideoFile(normRel); !ok {
		return nil, "", safefs.ErrForbiddenPath
	}
	videoInfo, _, err := root.safeRoot.Stat(normRel)
	if err != nil {
		return nil, "", err
	}
	if videoInfo.IsDir() {
		return nil, "", safefs.ErrForbiddenPath
	}

	// 1. Check for read-only sidecar image in the same folder
	if data, mime, ok := findSidecarImage(root.safeRoot, normRel); ok {
		return data, mime, nil
	}

	// 2. Check optional ffmpeg extraction into ephemeral cacheDir outside media roots
	if noThumbs || strings.TrimSpace(cacheDir) == "" || !HasFFmpeg() {
		return nil, "", ErrNoThumbnail
	}

	cacheKey := fmt.Sprintf("%s|%s|%d|%d", root.ID, normRel, videoInfo.Size(), videoInfo.ModTime().UnixNano())
	sum := sha256.Sum256([]byte(cacheKey))
	cacheFilename := hex.EncodeToString(sum[:]) + ".jpg"
	cacheFilePath := filepath.Join(cacheDir, cacheFilename)

	if cachedData, readErr := os.ReadFile(cacheFilePath); readErr == nil && len(cachedData) > 0 {
		return cachedData, mimeImageJPEG, nil
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, "", ErrNoThumbnail
	}

	extractCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	tmpFile, err := os.CreateTemp(cacheDir, ".thumb-*.jpg")
	if err != nil {
		return nil, "", ErrNoThumbnail
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)

	cmd, ok := NewFFmpegCommand(
		extractCtx,
		"-hide_banner",
		"-loglevel", "error",
		"-ss", "00:00:02",
		"-i", videoRealPath,
		"-frames:v", "1",
		"-vf", "scale=320:-1",
		"-q:v", "5",
		"-f", "image2",
		"-y",
		tmpPath,
	)
	if !ok {
		return nil, "", ErrNoThumbnail
	}
	if err := cmd.Run(); err != nil {
		return nil, "", ErrNoThumbnail
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil || len(data) == 0 {
		return nil, "", ErrNoThumbnail
	}

	_ = os.Rename(tmpPath, cacheFilePath)
	return data, mimeImageJPEG, nil
}

func findSidecarImage(sRoot *safefs.Root, videoNormRel string) ([]byte, string, bool) {
	ext := filepath.Ext(videoNormRel)
	baseRel := strings.TrimSuffix(videoNormRel, ext)

	for _, candidate := range sidecarImageExtensions {
		sidecarRel := baseRel + candidate.ext
		f, _, _, err := sRoot.OpenReadOnly(sidecarRel)
		if err != nil {
			continue
		}
		data, rErr := io.ReadAll(io.LimitReader(f, 5*1024*1024))
		_ = f.Close()
		if rErr == nil && len(data) > 0 {
			return data, candidate.mime, true
		}
	}
	return nil, "", false
}
