package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const CurrentSchemaVersion = 1

// SavedRootEntry represents a user-added media folder stored in library.json (FR-019).
type SavedRootEntry struct {
	Path    string `json:"path"`
	Label   string `json:"label,omitempty"`
	AddedAt string `json:"addedAt,omitempty"`
}

// LibraryConfig matches data-model.md (LibraryConfig.Version, LibraryConfig.Roots).
type LibraryConfig struct {
	Version int              `json:"version"`
	Roots   []SavedRootEntry `json:"roots"`
}

// Store manages atomic persistence of LibraryConfig outside any user media directory.
type Store struct {
	mu       sync.Mutex
	filePath string
}

// NewStore creates a new configuration store bound to filePath.
func NewStore(filePath string) *Store {
	return &Store{filePath: filePath}
}

// FilePath returns the path to the library.json file.
func (s *Store) FilePath() string {
	return s.filePath
}

// Load reads and parses LibraryConfig from disk. If the file does not exist yet,
// an empty v1 LibraryConfig is returned without error.
func (s *Store) Load() (*LibraryConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.filePath == "" {
		return &LibraryConfig{Version: CurrentSchemaVersion, Roots: []SavedRootEntry{}}, nil
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &LibraryConfig{Version: CurrentSchemaVersion, Roots: []SavedRootEntry{}}, nil
		}
		return nil, fmt.Errorf("falha ao ler arquivo de configuração %q: %w", s.filePath, err)
	}

	var cfg LibraryConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("arquivo de configuração %q inválido: %w", s.filePath, err)
	}
	if cfg.Version == 0 {
		cfg.Version = CurrentSchemaVersion
	}
	if cfg.Roots == nil {
		cfg.Roots = []SavedRootEntry{}
	}
	return &cfg, nil
}

// Save atomically writes LibraryConfig to disk (write temp file in same directory + os.Rename).
func (s *Store) Save(roots []SavedRootEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.filePath == "" {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	normalized := make([]SavedRootEntry, 0, len(roots))
	for _, r := range roots {
		if r.Path == "" {
			continue
		}
		if r.AddedAt == "" {
			r.AddedAt = now
		}
		normalized = append(normalized, r)
	}

	cfg := LibraryConfig{
		Version: CurrentSchemaVersion,
		Roots:   normalized,
	}

	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("falha ao serializar LibraryConfig: %w", err)
	}
	payload = append(payload, '\n')

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("falha ao criar diretório de configuração %q: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, ".library-*.json.tmp")
	if err != nil {
		return fmt.Errorf("falha ao criar arquivo temporário em %q: %w", dir, err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(payload); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("falha ao gravar arquivo temporário %q: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("falha ao fechar arquivo temporário %q: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("falha ao salvar %q atomicamente: %w", s.filePath, err)
	}
	return nil
}
