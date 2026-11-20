package catalog

import (
	"sort"
	"strings"
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
	c.mu.RLock()
	bRoot := c.browseBase
	baseLabel := "Diretório Base"
	if bRoot != nil {
		baseLabel = defaultFolderLabel(bRoot.CanonicalPath())
	}
	activeCanons := make(map[string]bool, len(c.roots))
	for _, r := range c.roots {
		activeCanons[r.CanonicalPath] = true
	}
	c.mu.RUnlock()

	if bRoot == nil {
		return nil, ErrBrowseBaseDisabled
	}

	entries, normRel, err := bRoot.ReadDir(relPath)
	if err != nil {
		return nil, err
	}

	dirs := make([]PickerDirEntry, 0)
	for _, entry := range entries {
		name := entry.Name()
		childRel := name
		if normRel != "" {
			childRel = normRel + "/" + name
		}
		targetCanon, _, rErr := bRoot.Resolve(childRel)
		if rErr != nil {
			continue
		}
		info, _, sErr := bRoot.Stat(childRel)
		if sErr != nil || !info.IsDir() {
			continue
		}

		hasSubdirs := false
		if subEntries, _, subErr := bRoot.ReadDir(childRel); subErr == nil {
			for _, sub := range subEntries {
				if sub.IsDir() {
					hasSubdirs = true
					break
				}
				subChildRel := childRel + "/" + sub.Name()
				if subInfo, _, stErr := bRoot.Stat(subChildRel); stErr == nil && subInfo.IsDir() {
					hasSubdirs = true
					break
				}
			}
		}

		dirs = append(dirs, PickerDirEntry{
			Name:         name,
			RelPath:      childRel,
			HasSubdirs:   hasSubdirs,
			AlreadyAdded: activeCanons[targetCanon],
		})
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	crumbs := buildBreadcrumbs("", baseLabel, normRel)
	return &PickerBrowseResult{
		BaseLabel:   baseLabel,
		CurrentPath: normRel,
		Breadcrumbs: crumbs,
		Directories: dirs,
	}, nil
}
