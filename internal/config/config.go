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

const (
	AppVersion = "0.1.0"
	appName    = "lan-video-player"
)

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
	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	fs.SetOutput(stderr)

	var mediaFlags multiStringFlag
	fs.Var(&mediaFlags, "media", "Caminho de pasta raiz de mídia (somente-leitura). Pode ser repetido.")

	defaultAddr := envOrDefault(getenv, "LVP_ADDR", "0.0.0.0:8080")
	envBrowseBase := strings.TrimSpace(getenv("LVP_BROWSE_BASE"))
	browseBaseExplicit := envBrowseBase != ""
	defaultBrowseBase := envBrowseBase
	if defaultBrowseBase == "" {
		defaultBrowseBase = detectDefaultBrowseBase()
	}

	defaultConfigFile := envOrDefault(getenv, "LVP_CONFIG_FILE", detectDefaultConfigFile())
	defaultCacheDir := envOrDefault(getenv, "LVP_CACHE_DIR", filepath.Join(os.TempDir(), "lan-video-player-thumbs"))
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
	if *showVersion {
		return &Config{ShowVersion: true}, nil
	}

	fs.Visit(func(f *flag.Flag) {
		if f.Name == "browse-base" {
			browseBaseExplicit = true
		}
	})

	validatedMedia, err := resolveAndValidateMediaDirs(mediaFlags, getenv("LVP_MEDIA_DIRS"))
	if err != nil {
		return nil, err
	}

	resolvedBrowseBase, err := resolveAndValidateBrowseBase(*browseBase, browseBaseExplicit)
	if err != nil {
		return nil, err
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

func envOrDefault(getenv func(string) string, key, fallback string) string {
	if val := strings.TrimSpace(getenv(key)); val != "" {
		return val
	}
	return fallback
}

func resolveAndValidateMediaDirs(cliMedia []string, envMediaRaw string) ([]string, error) {
	mediaDirs := append([]string{}, cliMedia...)
	if len(mediaDirs) == 0 && strings.TrimSpace(envMediaRaw) != "" {
		mediaDirs = append(mediaDirs, splitPathList(envMediaRaw)...)
	}

	validated := make([]string, 0, len(mediaDirs))
	for _, dir := range mediaDirs {
		root, err := safefs.NewRoot(dir)
		if err != nil {
			return nil, fmt.Errorf("pasta de mídia inválida %q: %w", dir, err)
		}
		validated = append(validated, root.CanonicalPath())
	}
	return validated, nil
}

func resolveAndValidateBrowseBase(rawBrowseBase string, explicit bool) (string, error) {
	trimmed := strings.TrimSpace(rawBrowseBase)
	if trimmed == "" {
		return "", nil
	}
	bRoot, err := safefs.NewRoot(trimmed)
	if err == nil {
		return bRoot.CanonicalPath(), nil
	}
	if explicit {
		return "", fmt.Errorf("diretório-base (--browse-base) inválido %q: %w", trimmed, err)
	}
	if cwdRoot, cwdErr := safefs.NewRoot("."); cwdErr == nil {
		return cwdRoot.CanonicalPath(), nil
	}
	return "", nil
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
		return filepath.Join(cfgDir, appName, "library.json")
	}
	return filepath.Join(os.TempDir(), appName, "library.json")
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
