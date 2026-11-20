package catalog

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cristiano/lan-video-player/internal/safefs"
)

// SubtitleTrack matches data-model.md 1.7 SubtitleTrack.
type SubtitleTrack struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Lang    string `json:"lang"`
	Format  string `json:"format"`
	RelPath string `json:"relPath"`
	SrcURL  string `json:"srcUrl"`
}

var srtTimestampRegex = regexp.MustCompile(`(\d{2}:\d{2}:\d{2}),(\d{3})`)

// IsSupportedSubtitleFile returns the normalized format ("vtt" or "srt") and true if supported.
func IsSupportedSubtitleFile(filename string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".vtt":
		return "vtt", true
	case ".srt":
		return "srt", true
	default:
		return "", false
	}
}

// DiscoverSubtitles scans directory entries in the same folder as videoFilename and returns
// matching .vtt and .srt tracks sharing the same base filename (FR-014).
func DiscoverSubtitles(root *safefs.Root, rootID string, folderRelPath string, videoFilename string, entries []os.DirEntry) []SubtitleTrack {
	videoExt := filepath.Ext(videoFilename)
	videoBase := strings.TrimSuffix(videoFilename, videoExt)
	lowerVideoBase := strings.ToLower(videoBase)

	tracks := make([]SubtitleTrack, 0)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		format, ok := IsSupportedSubtitleFile(name)
		if !ok {
			continue
		}

		subExt := filepath.Ext(name)
		subWithoutExt := strings.TrimSuffix(name, subExt)
		lowerSubWithoutExt := strings.ToLower(subWithoutExt)

		var suffix string
		if lowerSubWithoutExt == lowerVideoBase {
			suffix = ""
		} else if strings.HasPrefix(lowerSubWithoutExt, lowerVideoBase+".") {
			suffix = strings.TrimSpace(subWithoutExt[len(videoBase)+1:])
		} else {
			continue
		}

		relPath := name
		if folderRelPath != "" {
			relPath = folderRelPath + "/" + name
		}
		// Ensure subtitle file is valid inside safefs root
		if _, _, err := root.Resolve(relPath); err != nil {
			continue
		}

		label, lang := parseSubtitleLabelAndLang(suffix)
		encodedPath := EncodeRelPathForURL(relPath)
		tracks = append(tracks, SubtitleTrack{
			Label:   label,
			Lang:    lang,
			Format:  format,
			RelPath: relPath,
			SrcURL:  "/api/v1/subtitles/" + url.PathEscape(rootID) + "/" + encodedPath,
		})
	}

	sort.Slice(tracks, func(i, j int) bool {
		return strings.ToLower(tracks[i].Label) < strings.ToLower(tracks[j].Label)
	})

	for i := range tracks {
		tracks[i].ID = fmt.Sprintf("sub-%d", i)
	}
	return tracks
}

func parseSubtitleLabelAndLang(suffix string) (label string, lang string) {
	trimmed := strings.TrimSpace(suffix)
	if trimmed == "" {
		return "Padrão", "und"
	}
	return trimmed, trimmed
}

// ReadSubtitleAsWebVTT opens a .vtt or .srt file read-only from root and returns WebVTT bytes
// in memory without creating or modifying any files on disk (FR-008, FR-014).
func ReadSubtitleAsWebVTT(root *safefs.Root, relPath string) ([]byte, error) {
	format, ok := IsSupportedSubtitleFile(relPath)
	if !ok {
		return nil, safefs.ErrForbiddenPath
	}
	f, _, _, err := root.OpenReadOnly(relPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Limit subtitle read to 10 MB to prevent excessive memory use
	raw, err := io.ReadAll(io.LimitReader(f, 10*1024*1024))
	if err != nil {
		return nil, err
	}

	// Strip optional UTF-8 BOM
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})

	if format == "vtt" {
		return raw, nil
	}
	return ConvertSRTToWebVTT(raw), nil
}

// ConvertSRTToWebVTT converts SubRip (.srt) bytes into WebVTT (.vtt) format in memory.
func ConvertSRTToWebVTT(srtBytes []byte) []byte {
	text := strings.ReplaceAll(string(srtBytes), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	converted := srtTimestampRegex.ReplaceAllString(text, "${1}.${2}")
	trimmed := strings.TrimLeft(converted, "\n")
	var buf bytes.Buffer
	buf.WriteString("WEBVTT\n\n")
	buf.WriteString(trimmed)
	if !strings.HasSuffix(trimmed, "\n") {
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}
