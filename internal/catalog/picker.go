package catalog

import (
	"os"
	"sort"
	"strings"

	"github.com/cristiano/lan-video-player/internal/safefs"
)

// PickerDirEntry matches data-model.md 1.3 PickerDirEntry.
type PickerDirEntry struct {
	Name         string `json:"name"`
	RelPath      string `json:"relPath"`
	HasSubdirs   bool   `json:"hasSubdirs"`
	AlreadyAdded bool   `json:"alreadyAdded"`
}

// PickerBrowseResult represents the response payload of GET /api/v1/fs/dirs.
type PickerBrowseResult struct {
	BaseLabel   string           `json:"baseLabel"`
	CurrentPath string           `json:"currentPath"`
	Breadcrumbs []BreadcrumbItem `json:"breadcrumbs"`
	Directories []PickerDirEntry `json:"directories"`
}

// ListBrowseBaseDirs lists subdirectories inside --browse-base for the Web UI folder picker (FR-018).
func (c *Catalog) ListBrowseBaseDirs(relPath string) (*PickerBrowseResult, error) {
	bRoot, baseLabel, activeCanons := c.snapshotBrowseBaseState()
	if bRoot == nil {
		return nil, ErrBrowseBaseDisabled
	}

	entries, normRel, err := bRoot.ReadDir(relPath)
	if err != nil {
		return nil, err
	}

	dirs := make([]PickerDirEntry, 0, len(entries))
	for _, entry := range entries {
		item, ok := buildPickerDirEntry(bRoot, normRel, entry, activeCanons)
		if ok {
			dirs = append(dirs, item)
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	return &PickerBrowseResult{
		BaseLabel:   baseLabel,
		CurrentPath: normRel,
		Breadcrumbs: buildBreadcrumbs("", baseLabel, normRel),
		Directories: dirs,
	}, nil
}

func (c *Catalog) snapshotBrowseBaseState() (*safefs.Root, string, map[string]bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	bRoot := c.browseBase
	baseLabel := "Diretório Base"
	if bRoot != nil {
		baseLabel = defaultFolderLabel(bRoot.CanonicalPath())
	}
	activeCanons := make(map[string]bool, len(c.roots))
	for _, r := range c.roots {
		activeCanons[r.CanonicalPath] = true
	}
	return bRoot, baseLabel, activeCanons
}

func buildPickerDirEntry(bRoot *safefs.Root, normRel string, entry os.DirEntry, activeCanons map[string]bool) (PickerDirEntry, bool) {
	name := entry.Name()
	childRel := joinRelPath(normRel, name)

	targetCanon, _, err := bRoot.Resolve(childRel)
	if err != nil {
		return PickerDirEntry{}, false
	}
	info, _, err := bRoot.Stat(childRel)
	if err != nil || !info.IsDir() {
		return PickerDirEntry{}, false
	}

	return PickerDirEntry{
		Name:         name,
		RelPath:      childRel,
		HasSubdirs:   directoryHasSubdirs(bRoot, childRel),
		AlreadyAdded: activeCanons[targetCanon],
	}, true
}

func directoryHasSubdirs(bRoot *safefs.Root, dirRel string) bool {
	subEntries, _, err := bRoot.ReadDir(dirRel)
	if err != nil {
		return false
	}
	for _, sub := range subEntries {
		if sub.IsDir() {
			return true
		}
		subChildRel := joinRelPath(dirRel, sub.Name())
		if subInfo, _, stErr := bRoot.Stat(subChildRel); stErr == nil && subInfo.IsDir() {
			return true
		}
	}
	return false
}

func joinRelPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "/" + child
}
