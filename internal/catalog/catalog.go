package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cristiano/lan-video-player/internal/config"
	"github.com/cristiano/lan-video-player/internal/safefs"
)

var (
	// ErrRootNotFound indicates the requested MediaRoot ID does not exist.
	ErrRootNotFound = errors.New("media root not found")
	// ErrRootNotRemovable indicates an attempt to remove a CLI-configured root via the UI.
	ErrRootNotRemovable = errors.New("cli media root cannot be removed via UI")
	// ErrBrowseBaseDisabled indicates --browse-base is not configured.
	ErrBrowseBaseDisabled = errors.New("browse base directory is not enabled")
)

var supportedVideoExtensions = map[string]string{
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".webm": "video/webm",
	".mkv":  "video/x-matroska",
	".mov":  "video/quicktime",
	".ogv":  "video/ogg",
	".ogg":  "video/ogg",
	".avi":  "video/x-msvideo",
	".rmvb": "application/vnd.rn-realmedia-vbr",
	".rm":   "application/vnd.rn-realmedia",
	".wmv":  "video/x-ms-wmv",
	".flv":  "video/x-flv",
	".mpg":  "video/mpeg",
	".mpeg": "video/mpeg",
	".ts":   "video/mp2t",
	".m2ts": "video/mp2t",
}

var legacyTranscodeExtensions = map[string]bool{
	".avi":  true,
	".rmvb": true,
	".rm":   true,
	".wmv":  true,
	".flv":  true,
	".mpg":  true,
	".mpeg": true,
	".ts":   true,
	".m2ts": true,
}

// IsSupportedVideoFile returns the MIME type and true if filename has a supported video extension.
func IsSupportedVideoFile(filename string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(filename))
	mime, ok := supportedVideoExtensions[ext]
	return mime, ok
}

// RequiresTranscodeByDefault reports whether a video container format (e.g. .avi, .rmvb, .wmv)
// cannot be decoded natively by HTML5 browsers and should be transcoded on-the-fly when ffmpeg is available.
func RequiresTranscodeByDefault(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return legacyTranscodeExtensions[ext]
}

// MediaRoot matches data-model.md 1.1 MediaRoot.
type MediaRoot struct {
	ID              string       `json:"id"`
	Label           string       `json:"label"`
	CanonicalPath   string       `json:"-"`
	RelToBrowseBase string       `json:"relToBrowseBase,omitempty"`
	Source          string       `json:"source"`
	Removable       bool         `json:"removable"`
	safeRoot        *safefs.Root `json:"-"`
}

// SafeRoot returns the underlying read-only safefs.Root boundary.
func (m *MediaRoot) SafeRoot() *safefs.Root {
	return m.safeRoot
}

// BreadcrumbItem matches data-model.md 1.4 BreadcrumbItem.
type BreadcrumbItem struct {
	Label   string `json:"label"`
	RootID  string `json:"rootId,omitempty"`
	RelPath string `json:"relPath"`
}

// FolderEntry matches data-model.md 1.5 FolderEntry.
type FolderEntry struct {
	Name       string    `json:"name"`
	RootID     string    `json:"rootId"`
	RelPath    string    `json:"relPath"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

// VideoItem matches data-model.md 1.6 VideoItem.
type VideoItem struct {
	ID            string          `json:"id"`
	RootID        string          `json:"rootId"`
	RootLabel     string          `json:"rootLabel"`
	Name          string          `json:"name"`
	Title         string          `json:"title"`
	RelPath       string          `json:"relPath"`
	FolderRelPath string          `json:"folderRelPath"`
	SizeBytes     int64           `json:"sizeBytes"`
	ModifiedAt    time.Time       `json:"modifiedAt"`
	MimeType      string          `json:"mimeType"`
	StreamURL     string          `json:"streamUrl"`
	ThumbnailURL  string          `json:"thumbnailUrl"`
	Subtitles     []SubtitleTrack `json:"subtitles"`
}

// BrowseResult represents the response payload of GET /api/v1/browse.
type BrowseResult struct {
	Root        MediaRoot        `json:"root"`
	CurrentPath string           `json:"currentPath"`
	Breadcrumbs []BreadcrumbItem `json:"breadcrumbs"`
	Folders     []FolderEntry    `json:"folders"`
	Videos      []VideoItem      `json:"videos"`
}

// SearchResult represents the response payload of GET /api/v1/search.
type SearchResult struct {
	Query  string      `json:"query"`
	Total  int         `json:"total"`
	Videos []VideoItem `json:"videos"`
}

// Catalog manages the active read-only media roots, directory picker, browsing, and search.
type Catalog struct {
	mu         sync.RWMutex
	roots      []*MediaRoot
	browseBase *safefs.Root
	store      *config.Store
}

// New creates a Catalog initialized with CLI media directories, optional browse-base,
// and persisted UI folders from config.Store.
func New(cliMediaDirs []string, browseBaseDir string, store *config.Store) (*Catalog, error) {
	c := &Catalog{
		roots: make([]*MediaRoot, 0),
		store: store,
	}

	if strings.TrimSpace(browseBaseDir) != "" {
		bRoot, err := safefs.NewRoot(browseBaseDir)
		if err != nil {
			return nil, fmt.Errorf("browse-base inválido: %w", err)
		}
		c.browseBase = bRoot
	}

	// Register CLI roots first
	for _, dir := range cliMediaDirs {
		if err := c.registerRootLocked(dir, "cli"); err != nil {
			return nil, err
		}
	}

	// Load saved UI roots from store (FR-019)
	if store != nil {
		savedCfg, err := store.Load()
		if err != nil {
			return nil, err
		}
		for _, entry := range savedCfg.Roots {
			targetPath := entry.Path
			if c.browseBase != nil && !filepath.IsAbs(targetPath) {
				resolved, _, rErr := c.browseBase.Resolve(targetPath)
				if rErr == nil {
					targetPath = resolved
				}
			}
			// If a previously saved folder was removed externally from disk, skip gracefully
			// without crashing the entire server startup.
			if c.browseBase != nil {
				if _, ok := c.browseBase.RelFromRoot(targetPath); !ok {
					continue
				}
			}
			_ = c.registerRootLocked(targetPath, "ui")
		}
	}

	c.recomputeLabelsLocked()
	return c, nil
}

// BrowseBaseEnabled reports whether a --browse-base directory is active for the UI folder picker.
func (c *Catalog) BrowseBaseEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.browseBase != nil
}

// BrowseBaseLabel returns a friendly display name for the browse-base directory.
func (c *Catalog) BrowseBaseLabel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.browseBase == nil {
		return ""
	}
	base := filepath.Base(c.browseBase.CanonicalPath())
	if base == "" || base == "/" || base == "." {
		return "Diretório Base"
	}
	return base
}

// Roots returns a snapshot copy of all active MediaRoots.
func (c *Catalog) Roots() []MediaRoot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]MediaRoot, 0, len(c.roots))
	for _, r := range c.roots {
		out = append(out, *r)
	}
	return out
}

// GetRoot looks up an active MediaRoot by ID. If rootID is empty and only 1 root exists,
// that single root is returned automatically.
func (c *Catalog) GetRoot(rootID string) (*MediaRoot, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	trimmed := strings.TrimSpace(rootID)
	if trimmed == "" && len(c.roots) == 1 {
		copyRoot := *c.roots[0]
		return &copyRoot, nil
	}
	for _, r := range c.roots {
		if r.ID == trimmed {
			copyRoot := *r
			return &copyRoot, nil
		}
	}
	return nil, ErrRootNotFound
}

// AddUIRoots validates one or more relative paths inside --browse-base, adds them to the
// active library (idempotently), and persists the updated UI root list to library.json (FR-018, FR-019).
func (c *Catalog) AddUIRoots(relPaths []string) ([]MediaRoot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.browseBase == nil {
		return nil, ErrBrowseBaseDisabled
	}
	if len(relPaths) == 0 {
		return nil, errors.New("nenhuma pasta selecionada")
	}

	resolvedTargets := make([]string, 0, len(relPaths))
	for _, rel := range relPaths {
		targetReal, _, err := c.browseBase.Resolve(rel)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(targetReal)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, safefs.ErrNotFound
			}
			return nil, err
		}
		if !info.IsDir() {
			return nil, safefs.ErrNotDirectory
		}
		resolvedTargets = append(resolvedTargets, targetReal)
	}

	for _, targetReal := range resolvedTargets {
		if err := c.registerRootLocked(targetReal, "ui"); err != nil {
			return nil, err
		}
	}
	c.recomputeLabelsLocked()

	if err := c.persistUIRootsLocked(); err != nil {
		return nil, err
	}

	out := make([]MediaRoot, 0, len(c.roots))
	for _, r := range c.roots {
		out = append(out, *r)
	}
	return out, nil
}

// RemoveUIRoot unlinks a UI-added MediaRoot from the active library and updates library.json,
// without touching any file on disk (FR-018, FR-019).
func (c *Catalog) RemoveUIRoot(rootID string) ([]MediaRoot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	idx := -1
	for i, r := range c.roots {
		if r.ID == rootID {
			if !r.Removable || r.Source == "cli" {
				return nil, ErrRootNotRemovable
			}
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, ErrRootNotFound
	}

	c.roots = append(c.roots[:idx], c.roots[idx+1:]...)
	c.recomputeLabelsLocked()

	if err := c.persistUIRootsLocked(); err != nil {
		return nil, err
	}

	out := make([]MediaRoot, 0, len(c.roots))
	for _, r := range c.roots {
		out = append(out, *r)
	}
	return out, nil
}

func (c *Catalog) registerRootLocked(dirPath string, source string) error {
	sRoot, err := safefs.NewRoot(dirPath)
	if err != nil {
		return err
	}
	canon := sRoot.CanonicalPath()

	// Check if already registered (idempotent)
	for _, existing := range c.roots {
		if existing.CanonicalPath == canon {
			return nil
		}
	}

	id := generateRootID(canon, len(c.roots)+1)
	relToBrowse := ""
	if c.browseBase != nil {
		if rel, ok := c.browseBase.RelFromRoot(canon); ok {
			relToBrowse = rel
		}
	}

	c.roots = append(c.roots, &MediaRoot{
		ID:              id,
		Label:           defaultFolderLabel(canon),
		CanonicalPath:   canon,
		RelToBrowseBase: relToBrowse,
		Source:          source,
		Removable:       source == "ui",
		safeRoot:        sRoot,
	})
	return nil
}

func (c *Catalog) recomputeLabelsLocked() {
	counts := make(map[string]int)
	baseNames := make([]string, len(c.roots))
	for i, r := range c.roots {
		base := defaultFolderLabel(r.CanonicalPath)
		baseNames[i] = base
		counts[strings.ToLower(base)]++
	}

	seen := make(map[string]int)
	for i, r := range c.roots {
		base := baseNames[i]
		key := strings.ToLower(base)
		if counts[key] > 1 {
			seen[key]++
			r.Label = fmt.Sprintf("%s (%d)", base, seen[key])
		} else {
			r.Label = base
		}
	}
}

func (c *Catalog) persistUIRootsLocked() error {
	if c.store == nil {
		return nil
	}
	saved := make([]config.SavedRootEntry, 0)
	for _, r := range c.roots {
		if r.Source == "ui" {
			saved = append(saved, config.SavedRootEntry{
				Path:  r.CanonicalPath,
				Label: r.Label,
			})
		}
	}
	return c.store.Save(saved)
}

func defaultFolderLabel(canonPath string) string {
	base := filepath.Base(canonPath)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "Biblioteca"
	}
	return base
}

func generateRootID(canonicalPath string, index int) string {
	sum := sha256.Sum256([]byte(canonicalPath))
	shortHash := hex.EncodeToString(sum[:])[:6]
	return fmt.Sprintf("root-%d-%s", index, shortHash)
}

// Browse lists subfolders and videos inside a specific MediaRoot and relative directory path.
func (c *Catalog) Browse(rootID string, relPath string, sortBy string) (*BrowseResult, error) {
	root, err := c.GetRoot(rootID)
	if err != nil {
		return nil, err
	}

	entries, normRel, err := root.safeRoot.ReadDir(relPath)
	if err != nil {
		return nil, err
	}

	folders := make([]FolderEntry, 0)
	videos := make([]VideoItem, 0)

	for _, entry := range entries {
		name := entry.Name()
		childRel := name
		if normRel != "" {
			childRel = normRel + "/" + name
		}

		info, _, sErr := root.safeRoot.Stat(childRel)
		if sErr != nil {
			continue
		}

		if info.IsDir() {
			folders = append(folders, FolderEntry{
				Name:       name,
				RootID:     root.ID,
				RelPath:    childRel,
				ModifiedAt: info.ModTime().UTC(),
			})
			continue
		}

		mimeType, isVideo := IsSupportedVideoFile(name)
		if !isVideo {
			continue
		}

		subs := DiscoverSubtitles(root.safeRoot, root.ID, normRel, name, entries)
		videos = append(videos, buildVideoItem(root, name, childRel, normRel, info, mimeType, subs))
	}

	sortFolders(folders, sortBy)
	sortVideos(videos, sortBy)

	return &BrowseResult{
		Root:        *root,
		CurrentPath: normRel,
		Breadcrumbs: buildBreadcrumbs(root.ID, root.Label, normRel),
		Folders:     folders,
		Videos:      videos,
	}, nil
}

// Search performs a recursive case-insensitive search by video title/filename across all active
// roots (or a single filtered rootID) and sorts the results (FR-011).
func (c *Catalog) Search(query string, rootID string, sortBy string) (*SearchResult, error) {
	trimmedQuery := strings.TrimSpace(query)
	lowerQuery := strings.ToLower(trimmedQuery)

	var targetRoots []MediaRoot
	if strings.TrimSpace(rootID) != "" {
		r, err := c.GetRoot(rootID)
		if err != nil {
			return nil, err
		}
		targetRoots = []MediaRoot{*r}
	} else {
		targetRoots = c.Roots()
	}

	matches := make([]VideoItem, 0)
	if lowerQuery == "" {
		return &SearchResult{Query: trimmedQuery, Total: 0, Videos: matches}, nil
	}

	for i := range targetRoots {
		root := &targetRoots[i]
		c.walkRootForSearch(root, "", lowerQuery, &matches)
	}

	sortVideos(matches, sortBy)
	return &SearchResult{
		Query:  trimmedQuery,
		Total:  len(matches),
		Videos: matches,
	}, nil
}

func (c *Catalog) walkRootForSearch(root *MediaRoot, dirRel string, lowerQuery string, out *[]VideoItem) {
	entries, normRel, err := root.safeRoot.ReadDir(dirRel)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		childRel := name
		if normRel != "" {
			childRel = normRel + "/" + name
		}
		info, _, sErr := root.safeRoot.Stat(childRel)
		if sErr != nil {
			continue
		}
		if info.IsDir() {
			c.walkRootForSearch(root, childRel, lowerQuery, out)
			continue
		}
		mimeType, isVideo := IsSupportedVideoFile(name)
		if !isVideo {
			continue
		}
		if strings.Contains(strings.ToLower(name), lowerQuery) {
			subs := DiscoverSubtitles(root.safeRoot, root.ID, normRel, name, entries)
			*out = append(*out, buildVideoItem(root, name, childRel, normRel, info, mimeType, subs))
		}
	}
}

func buildVideoItem(root *MediaRoot, filename, relPath, folderRel string, info os.FileInfo, mimeType string, subs []SubtitleTrack) VideoItem {
	ext := filepath.Ext(filename)
	title := strings.TrimSuffix(filename, ext)
	encodedPath := EncodeRelPathForURL(relPath)
	if subs == nil {
		subs = []SubtitleTrack{}
	}
	return VideoItem{
		ID:            root.ID + ":" + relPath,
		RootID:        root.ID,
		RootLabel:     root.Label,
		Name:          filename,
		Title:         title,
		RelPath:       relPath,
		FolderRelPath: folderRel,
		SizeBytes:     info.Size(),
		ModifiedAt:    info.ModTime().UTC(),
		MimeType:      mimeType,
		StreamURL:     "/api/v1/stream/" + url.PathEscape(root.ID) + "/" + encodedPath,
		ThumbnailURL:  "/api/v1/thumbnails/" + url.PathEscape(root.ID) + "/" + encodedPath,
		Subtitles:     subs,
	}
}

func buildBreadcrumbs(rootID, rootLabel, normRel string) []BreadcrumbItem {
	crumbs := []BreadcrumbItem{
		{
			Label:   rootLabel,
			RootID:  rootID,
			RelPath: "",
		},
	}
	if normRel == "" {
		return crumbs
	}
	parts := strings.Split(normRel, "/")
	acc := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if acc == "" {
			acc = p
		} else {
			acc = acc + "/" + p
		}
		crumbs = append(crumbs, BreadcrumbItem{
			Label:   p,
			RootID:  rootID,
			RelPath: acc,
		})
	}
	return crumbs
}

// EncodeRelPathForURL escapes each path segment individually while preserving '/' separators.
func EncodeRelPathForURL(relPath string) string {
	clean := path.Clean(strings.TrimPrefix(relPath, "/"))
	if clean == "." || clean == "" {
		return ""
	}
	segments := strings.Split(clean, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	return strings.Join(segments, "/")
}

func sortFolders(folders []FolderEntry, sortBy string) {
	sort.Slice(folders, func(i, j int) bool {
		switch sortBy {
		case "name_desc":
			return strings.ToLower(folders[i].Name) > strings.ToLower(folders[j].Name)
		case "date_desc":
			if !folders[i].ModifiedAt.Equal(folders[j].ModifiedAt) {
				return folders[i].ModifiedAt.After(folders[j].ModifiedAt)
			}
			return strings.ToLower(folders[i].Name) < strings.ToLower(folders[j].Name)
		case "date_asc":
			if !folders[i].ModifiedAt.Equal(folders[j].ModifiedAt) {
				return folders[i].ModifiedAt.Before(folders[j].ModifiedAt)
			}
			return strings.ToLower(folders[i].Name) < strings.ToLower(folders[j].Name)
		default: // name_asc
			return strings.ToLower(folders[i].Name) < strings.ToLower(folders[j].Name)
		}
	})
}

func sortVideos(videos []VideoItem, sortBy string) {
	sort.Slice(videos, func(i, j int) bool {
		switch sortBy {
		case "name_desc":
			return strings.ToLower(videos[i].Name) > strings.ToLower(videos[j].Name)
		case "date_desc":
			if !videos[i].ModifiedAt.Equal(videos[j].ModifiedAt) {
				return videos[i].ModifiedAt.After(videos[j].ModifiedAt)
			}
			return strings.ToLower(videos[i].Name) < strings.ToLower(videos[j].Name)
		case "date_asc":
			if !videos[i].ModifiedAt.Equal(videos[j].ModifiedAt) {
				return videos[i].ModifiedAt.Before(videos[j].ModifiedAt)
			}
			return strings.ToLower(videos[i].Name) < strings.ToLower(videos[j].Name)
		default: // name_asc
			return strings.ToLower(videos[i].Name) < strings.ToLower(videos[j].Name)
		}
	})
}
