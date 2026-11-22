package catalog

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

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

var (
	srtTimestampRegex = regexp.MustCompile(`(\d{1,2}:\d{2}(?::\d{2})?),(\d{3})`)
	vttCueRegex       = regexp.MustCompile(`(\d{1,2}:\d{2}(?::\d{2})?\.\d{3})\s*-->\s*(\d{1,2}:\d{2}(?::\d{2})?\.\d{3})`)
)

// win1252Extra maps Windows-1252 bytes 0x80..0x9F to Unicode runes.
var win1252Extra = [32]rune{
	0x20AC, 0x0081, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x008D, 0x017D, 0x008F,
	0x0090, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x009D, 0x017E, 0x0178,
}

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

	// Convert ANSI (Windows-1252 / ISO-8859-1) to UTF-8 if needed, then strip optional UTF-8 BOM.
	utf8Raw := ensureUTF8(raw)
	utf8Raw = bytes.TrimPrefix(utf8Raw, []byte{0xEF, 0xBB, 0xBF})

	if format == "vtt" {
		return normalizeWebVTT(utf8Raw), nil
	}
	return ConvertSRTToWebVTT(utf8Raw), nil
}

// ConvertSRTToWebVTT converts SubRip (.srt) bytes into WebVTT (.vtt) format in memory.
func ConvertSRTToWebVTT(srtBytes []byte) []byte {
	utf8Bytes := bytes.TrimPrefix(ensureUTF8(srtBytes), []byte{0xEF, 0xBB, 0xBF})
	text := strings.ReplaceAll(string(utf8Bytes), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	converted := srtTimestampRegex.ReplaceAllString(text, "${1}.${2}")
	trimmed := strings.TrimSpace(converted)
	var buf bytes.Buffer
	buf.WriteString("WEBVTT\n\n")
	if trimmed != "" {
		buf.WriteString(trimmed)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// ShiftWebVTTTimestamps shifts all WebVTT cue timestamps by shiftSeconds.
// Positive shiftSeconds delays subtitles; negative advances them (e.g. transcoded seek offset).
func ShiftWebVTTTimestamps(vttBytes []byte, shiftSeconds float64) []byte {
	if math.Abs(shiftSeconds) < 0.001 {
		return vttBytes
	}
	shifted := vttCueRegex.ReplaceAllStringFunc(string(vttBytes), func(match string) string {
		parts := vttCueRegex.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		rawStart := parseVTTTime(parts[1]) + shiftSeconds
		rawEnd := parseVTTTime(parts[2]) + shiftSeconds
		start := math.Max(0.0, rawStart)
		end := math.Max(start+0.1, rawEnd)
		return formatVTTTime(start) + " --> " + formatVTTTime(end)
	})
	return []byte(shifted)
}

func ensureUTF8(raw []byte) []byte {
	if utf8.Valid(raw) {
		return raw
	}
	var b strings.Builder
	b.Grow(len(raw))
	for _, ch := range raw {
		if ch < 0x80 {
			b.WriteByte(ch)
		} else if ch <= 0x9F {
			b.WriteRune(win1252Extra[ch-0x80])
		} else {
			b.WriteRune(rune(ch))
		}
	}
	return []byte(b.String())
}

func normalizeWebVTT(vttBytes []byte) []byte {
	text := strings.ReplaceAll(string(vttBytes), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(strings.ToUpper(trimmed), "WEBVTT") {
		if trimmed == "" {
			return []byte("WEBVTT\n\n")
		}
		return []byte("WEBVTT\n\n" + trimmed + "\n")
	}
	return []byte(trimmed + "\n")
}

func parseVTTTime(vttTime string) float64 {
	hms, msStr, _ := strings.Cut(vttTime, ".")
	parts := strings.Split(hms, ":")
	var hours, minutes, seconds int
	switch len(parts) {
	case 3:
		hours, _ = strconv.Atoi(parts[0])
		minutes, _ = strconv.Atoi(parts[1])
		seconds, _ = strconv.Atoi(parts[2])
	case 2:
		minutes, _ = strconv.Atoi(parts[0])
		seconds, _ = strconv.Atoi(parts[1])
	}
	ms, _ := strconv.Atoi(msStr)
	return float64(hours*3600+minutes*60+seconds) + float64(ms)/1000.0
}

func formatVTTTime(totalSeconds float64) string {
	clamped := math.Max(0.0, totalSeconds)
	whole := int(math.Floor(clamped))
	ms := int(math.Round((clamped - float64(whole)) * 1000.0))
	if ms >= 1000 {
		whole++
		ms -= 1000
	}
	hours := whole / 3600
	minutes := (whole % 3600) / 60
	seconds := whole % 60
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, ms)
}
