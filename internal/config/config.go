package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristiano/lan-video-player/internal/safefs"
)

const AppVersion = "0.1.0"

// multiStringFlag allows repeating --media on the command line.
type multiStringFlag []string

func (m *multiStringFlag) String() string {
	return strings.Join(*m, string(os.PathListSeparator))
}

func (m *multiStringFlag) Set(val string) error {
	trimmed := strings.TrimSpace(val)
	if trimmed != "" {
		*m = append(*m, trimmed)
	}
	return nil
}

// Config holds the validated startup configuration of lan-video-player.
type Config struct {
	Addr        string
	MediaDirs   []string
	BrowseBase  string
	ConfigFile  string
	CacheDir    string
	NoThumbs    bool
	ShowVersion bool
}

// Parse parses CLI arguments and environment variables (LVP_*), validating that all configured
// media roots and browse-base directories exist and are readable (FR-001, FR-002).
func Parse(args []string, getenv func(string) string, stderr io.Writer) (*Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	fs := flag.NewFlagSet("lan-video-player", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var mediaFlags multiStringFlag
	fs.Var(&mediaFlags, "media", "Caminho de pasta raiz de mídia (somente-leitura). Pode ser repetido.")

	defaultAddr := strings.TrimSpace(getenv("LVP_ADDR"))
	if defaultAddr == "" {
		defaultAddr = "0.0.0.0:8080"
	}

	defaultBrowseBase := strings.TrimSpace(getenv("LVP_BROWSE_BASE"))
	browseBaseExplicit := defaultBrowseBase != ""
	if defaultBrowseBase == "" {
		defaultBrowseBase = detectDefaultBrowseBase()
	}

	defaultConfigFile := strings.TrimSpace(getenv("LVP_CONFIG_FILE"))
	if defaultConfigFile == "" {
		defaultConfigFile = detectDefaultConfigFile()
	}

	defaultCacheDir := strings.TrimSpace(getenv("LVP_CACHE_DIR"))
	if defaultCacheDir == "" {
		defaultCacheDir = filepath.Join(os.TempDir(), "lan-video-player-thumbs")
	}

	defaultNoThumbs := parseBoolEnv(getenv("LVP_NO_THUMBS"))

	addr := fs.String("addr", defaultAddr, "Endereço e porta HTTP de escuta na LAN (ex.: 0.0.0.0:8080)")
	browseBase := fs.String("browse-base", defaultBrowseBase, "Diretório-base seguro para o seletor visual de pastas na Web UI")
	configFile := fs.String("config-file", defaultConfigFile, "Arquivo JSON local (fora das pastas de mídia) para salvar pastas adicionadas na Web UI")
	cacheDir := fs.String("cache-dir", defaultCacheDir, "Diretório temporário fora da mídia para cache de miniaturas")
	noThumbs := fs.Bool("no-thumbs", defaultNoThumbs, "Desativa extração de miniaturas no servidor via ffmpeg")
	showVersion := fs.Bool("version", false, "Exibe a versão e a licença (GNU AGPL v3.0)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// Track if --browse-base was explicitly passed via CLI flag
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "browse-base" {
			browseBaseExplicit = true
		}
	})

	if *showVersion {
		return &Config{ShowVersion: true}, nil
	}

	// Merge CLI --media flags with LVP_MEDIA_DIRS when no CLI flags are provided
	mediaDirs := make([]string, 0, len(mediaFlags))
	mediaDirs = append(mediaDirs, mediaFlags...)
	if len(mediaDirs) == 0 {
		envMedia := strings.TrimSpace(getenv("LVP_MEDIA_DIRS"))
		if envMedia != "" {
			for _, part := range splitPathList(envMedia) {
				if part != "" {
					mediaDirs = append(mediaDirs, part)
				}
			}
		}
	}

	// Validate each configured media directory strictly (FR-002)
	validatedMedia := make([]string, 0, len(mediaDirs))
	for _, dir := range mediaDirs {
		root, err := safefs.NewRoot(dir)
		if err != nil {
			return nil, fmt.Errorf("pasta de mídia inválida %q: %w", dir, err)
		}
		validatedMedia = append(validatedMedia, root.CanonicalPath())
	}

	// Validate browseBase directory
	resolvedBrowseBase := strings.TrimSpace(*browseBase)
	if resolvedBrowseBase != "" {
		bRoot, err := safefs.NewRoot(resolvedBrowseBase)
		if err != nil {
			if browseBaseExplicit {
				return nil, fmt.Errorf("diretório-base (--browse-base) inválido %q: %w", resolvedBrowseBase, err)
			}
			// Fallback to current working directory if implicit default was unavailable
			if cwdRoot, cwdErr := safefs.NewRoot("."); cwdErr == nil {
				resolvedBrowseBase = cwdRoot.CanonicalPath()
			} else {
				resolvedBrowseBase = ""
			}
		} else {
			resolvedBrowseBase = bRoot.CanonicalPath()
		}
	}

	return &Config{
		Addr:        strings.TrimSpace(*addr),
		MediaDirs:   validatedMedia,
		BrowseBase:  resolvedBrowseBase,
		ConfigFile:  strings.TrimSpace(*configFile),
		CacheDir:    strings.TrimSpace(*cacheDir),
		NoThumbs:    *noThumbs,
		ShowVersion: false,
	}, nil
}

func detectDefaultBrowseBase() string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		if info, sErr := os.Stat(home); sErr == nil && info.IsDir() {
			return home
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func detectDefaultConfigFile() string {
	if cfgDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(cfgDir) != "" {
		return filepath.Join(cfgDir, "lan-video-player", "library.json")
	}
	return filepath.Join(os.TempDir(), "lan-video-player", "library.json")
}

func parseBoolEnv(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func splitPathList(raw string) []string {
	sep := string(os.PathListSeparator)
	parts := strings.Split(raw, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
