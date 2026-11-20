package catalog

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	hostLdLinux = "/var/run/host/usr/lib64/ld-linux-x86-64.so.2"
	hostLibDir  = "/var/run/host/usr/lib64"
	hostFFmpeg  = "/var/run/host/usr/bin/ffmpeg"
	hostFFprobe = "/var/run/host/usr/bin/ffprobe"
)

var durationCache sync.Map

// HasFFmpeg reports whether ffmpeg is available either directly in PATH or via the host mount.
func HasFFmpeg() bool {
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		return true
	}
	if fileExists(hostLdLinux) && fileExists(hostFFmpeg) {
		return true
	}
	return false
}

// NewFFmpegCommand constructs an *exec.Cmd for ffmpeg, supporting both standard PATH
// execution (native host or Podman/Docker container) and Flatpak sandbox host mounts.
func NewFFmpegCommand(ctx context.Context, args ...string) (*exec.Cmd, bool) {
	if ffmpegPath, err := exec.LookPath("ffmpeg"); err == nil {
		return exec.CommandContext(ctx, ffmpegPath, args...), true
	}
	if fileExists(hostLdLinux) && fileExists(hostFFmpeg) {
		fullArgs := make([]string, 0, len(args)+3)
		fullArgs = append(fullArgs, "--library-path", hostLibDir, hostFFmpeg)
		fullArgs = append(fullArgs, args...)
		return exec.CommandContext(ctx, hostLdLinux, fullArgs...), true
	}
	return nil, false
}

// NewFFprobeCommand constructs an *exec.Cmd for ffprobe, supporting both standard PATH
// execution and Flatpak sandbox host mounts.
func NewFFprobeCommand(ctx context.Context, args ...string) (*exec.Cmd, bool) {
	if ffprobePath, err := exec.LookPath("ffprobe"); err == nil {
		return exec.CommandContext(ctx, ffprobePath, args...), true
	}
	if fileExists(hostLdLinux) && fileExists(hostFFprobe) {
		fullArgs := make([]string, 0, len(args)+3)
		fullArgs = append(fullArgs, "--library-path", hostLibDir, hostFFprobe)
		fullArgs = append(fullArgs, args...)
		return exec.CommandContext(ctx, hostLdLinux, fullArgs...), true
	}
	return nil, false
}

// ProbeResult holds media duration and transcode metadata for a video file.
type ProbeResult struct {
	DurationSeconds   float64 `json:"durationSeconds"`
	RequiresTranscode bool    `json:"requiresTranscode"`
}

// ProbeVideo returns the duration in seconds (via ffprobe when available) and whether
// the file container requires on-the-fly transcoding for browser playback.
func (c *Catalog) ProbeVideo(ctx context.Context, rootID string, videoRelPath string) (*ProbeResult, error) {
	root, err := c.GetRoot(rootID)
	if err != nil {
		return nil, err
	}
	targetReal, normRel, err := root.safeRoot.Resolve(videoRelPath)
	if err != nil {
		return nil, err
	}
	if _, ok := IsSupportedVideoFile(normRel); !ok {
		return nil, os.ErrPermission
	}
	info, _, err := root.safeRoot.Stat(normRel)
	if err != nil {
		return nil, err
	}

	reqTranscode := RequiresTranscodeByDefault(normRel) && HasFFmpeg()
	cacheKey := fmt.Sprintf("%s|%d|%d", targetReal, info.Size(), info.ModTime().UnixNano())
	if cached, ok := durationCache.Load(cacheKey); ok {
		return &ProbeResult{
			DurationSeconds:   cached.(float64),
			RequiresTranscode: reqTranscode,
		}, nil
	}

	probeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	cmd, ok := NewFFprobeCommand(
		probeCtx,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		targetReal,
	)
	if !ok {
		return &ProbeResult{
			DurationSeconds:   0,
			RequiresTranscode: reqTranscode,
		}, nil
	}

	out, err := cmd.Output()
	if err != nil {
		return &ProbeResult{
			DurationSeconds:   0,
			RequiresTranscode: reqTranscode,
		}, nil
	}

	dur, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || dur < 0 {
		dur = 0
	} else {
		durationCache.Store(cacheKey, dur)
	}

	return &ProbeResult{
		DurationSeconds:   dur,
		RequiresTranscode: reqTranscode,
	}, nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
