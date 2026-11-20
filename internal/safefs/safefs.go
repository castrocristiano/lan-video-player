package safefs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrForbiddenPath indicates a path traversal attempt, hidden file access, or symlink escaping the root.
	ErrForbiddenPath = errors.New("forbidden path outside authorized root")
	// ErrNotFound indicates the requested path does not exist inside the authorized root.
	ErrNotFound = errors.New("path not found")
	// ErrNotDirectory indicates a path expected to be a directory is not a directory.
	ErrNotDirectory = errors.New("path is not a directory")
)

// Root represents a strictly read-only, canonicalized directory boundary on the filesystem.
type Root struct {
	canonicalPath string
}

// NewRoot validates that dirPath exists, is a readable directory, and resolves its canonical path
// (evaluating all symlinks).
func NewRoot(dirPath string) (*Root, error) {
	trimmed := strings.TrimSpace(dirPath)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: empty directory path", ErrNotDirectory)
	}
	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %q: %w", dirPath, err)
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, dirPath)
		}
		return nil, fmt.Errorf("failed to evaluate symlinks for %q: %w", dirPath, err)
	}
	info, err := os.Stat(realPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, dirPath)
		}
		return nil, fmt.Errorf("failed to stat %q: %w", dirPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrNotDirectory, dirPath)
	}
	// Verify read permission on the directory
	f, err := os.Open(realPath)
	if err != nil {
		return nil, fmt.Errorf("directory %q is not readable: %w", dirPath, err)
	}
	_ = f.Close()

	return &Root{canonicalPath: filepath.Clean(realPath)}, nil
}

// CanonicalPath returns the resolved absolute directory path of the root.
func (r *Root) CanonicalPath() string {
	return r.canonicalPath
}

// IsHiddenName returns true if the base name or any path segment starts with a dot ('.').
func IsHiddenName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || name == "." {
		return false
	}
	return strings.HasPrefix(name, ".")
}

// Resolve validates a relative path against the Root, blocking "..", null bytes, hidden segments,
// and symlinks that resolve outside r.canonicalPath. It returns the canonical target path and
// the normalized slash-separated relative path inside the root.
func (r *Root) Resolve(relPath string) (targetReal string, normalizedRel string, err error) {
	if r == nil || r.canonicalPath == "" || strings.ContainsRune(relPath, '\x00') {
		return "", "", ErrForbiddenPath
	}

	cleanSegments, err := parseSafeRelativeSegments(relPath)
	if err != nil {
		return "", "", err
	}
	if len(cleanSegments) == 0 {
		return r.canonicalPath, "", nil
	}

	joinedCandidate := filepath.Join(append([]string{r.canonicalPath}, cleanSegments...)...)
	if !isWithinBase(r.canonicalPath, joinedCandidate) {
		return "", "", ErrForbiddenPath
	}

	evalTarget, err := r.evalCandidateSymlinks(joinedCandidate)
	if err != nil {
		return "", "", err
	}

	return evalTarget, strings.Join(cleanSegments, "/"), nil
}

func parseSafeRelativeSegments(relPath string) ([]string, error) {
	slashPath := strings.ReplaceAll(strings.TrimSpace(relPath), "\\", "/")
	slashPath = strings.TrimPrefix(slashPath, "/")
	if slashPath == "" || slashPath == "." {
		return nil, nil
	}

	rawSegments := strings.Split(slashPath, "/")
	cleanSegments := make([]string, 0, len(rawSegments))
	for _, seg := range rawSegments {
		if seg == "" || seg == "." {
			continue
		}
		if seg == ".." || strings.HasPrefix(seg, ".") {
			return nil, ErrForbiddenPath
		}
		cleanSegments = append(cleanSegments, seg)
	}
	return cleanSegments, nil
}

func (r *Root) evalCandidateSymlinks(joinedCandidate string) (string, error) {
	evalTarget, err := filepath.EvalSymlinks(joinedCandidate)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			parentDir := filepath.Dir(joinedCandidate)
			if evalParent, pErr := filepath.EvalSymlinks(parentDir); pErr == nil && !isWithinBase(r.canonicalPath, evalParent) {
				return "", ErrForbiddenPath
			}
			return "", ErrNotFound
		}
		return "", ErrForbiddenPath
	}

	evalTarget = filepath.Clean(evalTarget)
	if !isWithinBase(r.canonicalPath, evalTarget) {
		return "", ErrForbiddenPath
	}
	return evalTarget, nil
}

// RelFromRoot computes the normalized slash-separated relative path of targetCanonical inside r,
// returning ok=false if targetCanonical is outside r.
func (r *Root) RelFromRoot(targetCanonical string) (string, bool) {
	if r == nil || r.canonicalPath == "" {
		return "", false
	}
	realTarget, err := filepath.EvalSymlinks(targetCanonical)
	if err != nil {
		return "", false
	}
	realTarget = filepath.Clean(realTarget)
	if !isWithinBase(r.canonicalPath, realTarget) {
		return "", false
	}
	rel, err := filepath.Rel(r.canonicalPath, realTarget)
	if err != nil {
		return "", false
	}
	if rel == "." {
		return "", true
	}
	return filepath.ToSlash(rel), true
}

// Stat returns the os.FileInfo for a validated relative path inside Root.
func (r *Root) Stat(relPath string) (os.FileInfo, string, error) {
	target, normRel, err := r.Resolve(relPath)
	if err != nil {
		return nil, "", err
	}
	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	return info, normRel, nil
}

// ReadDir lists directory entries inside a validated relative directory path of Root,
// automatically filtering out hidden entries (starting with '.') and any symlinks that
// point outside Root or are broken.
func (r *Root) ReadDir(relPath string) ([]os.DirEntry, string, error) {
	target, normRel, err := r.Resolve(relPath)
	if err != nil {
		return nil, "", err
	}
	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	if !info.IsDir() {
		return nil, "", ErrNotDirectory
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, "", err
	}

	safeEntries := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if IsHiddenName(name) {
			continue
		}
		childRel := name
		if normRel != "" {
			childRel = normRel + "/" + name
		}
		if _, _, err := r.Resolve(childRel); err != nil {
			// Skip broken symlinks or symlinks pointing outside the authorized root
			continue
		}
		safeEntries = append(safeEntries, entry)
	}
	return safeEntries, normRel, nil
}

// OpenReadOnly opens a regular file inside Root strictly in read-only mode (O_RDONLY).
func (r *Root) OpenReadOnly(relPath string) (*os.File, os.FileInfo, string, error) {
	target, normRel, err := r.Resolve(relPath)
	if err != nil {
		return nil, nil, "", err
	}
	f, err := os.OpenFile(target, os.O_RDONLY, 0)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, "", ErrNotFound
		}
		return nil, nil, "", err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, "", err
	}
	if info.IsDir() {
		_ = f.Close()
		return nil, nil, "", fmt.Errorf("cannot open directory as file: %s", normRel)
	}
	return f, info, normRel, nil
}

func isWithinBase(baseCanonical, targetCanonical string) bool {
	base := filepath.Clean(baseCanonical)
	target := filepath.Clean(targetCanonical)
	if base == target {
		return true
	}
	prefix := base
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(target, prefix)
}
