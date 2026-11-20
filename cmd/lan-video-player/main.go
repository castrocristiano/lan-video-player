package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cristiano/lan-video-player/internal/catalog"
	"github.com/cristiano/lan-video-player/internal/config"
	"github.com/cristiano/lan-video-player/internal/server"
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv))
}

func run(args []string, getenv func(string) string) int {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Parse(args, getenv, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro de configuração: %v\n", err)
		return 1
	}

	if cfg.ShowVersion {
		fmt.Printf("lan-video-player v%s\nLicença: GNU Affero General Public License v3.0 (GNU AGPL v3.0)\n", config.AppVersion)
		return 0
	}

	store := config.NewStore(cfg.ConfigFile)
	cat, err := catalog.New(cfg.MediaDirs, cfg.BrowseBase, store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao inicializar catálogo de mídia: %v\n", err)
		return 1
	}

	srvHandler, err := server.New(cat, cfg.CacheDir, cfg.NoThumbs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao inicializar servidor HTTP: %v\n", err)
		return 1
	}

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srvHandler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao abrir porta HTTP %q: %v\n", cfg.Addr, err)
		return 1
	}

	printStartupBanner(ln.Addr().String(), cat, cfg)

	errCh := make(chan error, 1)
	go func() {
		if sErr := httpServer.Serve(ln); sErr != nil && !errors.Is(sErr, http.ErrServerClosed) {
			errCh <- sErr
		}
		close(errCh)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sErr := <-errCh:
		if sErr != nil {
			logger.Error("Falha no servidor HTTP", "error", sErr)
			return 1
		}
	case sig := <-sigCh:
		logger.Info("Encerrando lan-video-player...", "signal", sig.String())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}
	return 0
}

func printStartupBanner(listenAddr string, cat *catalog.Catalog, cfg *config.Config) {
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		port = "8080"
	}

	fmt.Println("============================================================")
	fmt.Printf("  lan-video-player v%s (GNU AGPL v3.0) — 100%% Offline LAN\n", config.AppVersion)
	fmt.Println("============================================================")
	fmt.Printf("  Local:      http://localhost:%s\n", port)
	for _, ip := range detectLANIPv4s() {
		fmt.Printf("  Rede LAN:   http://%s:%s\n", ip, port)
	}
	if cfg.BrowseBase != "" {
		fmt.Printf("  Base UI:    %s (seletor visual de pastas)\n", cfg.BrowseBase)
	}
	if cfg.ConfigFile != "" {
		fmt.Printf("  Config UI:  %s\n", cfg.ConfigFile)
	}
	roots := cat.Roots()
	if len(roots) == 0 {
		fmt.Println("  Biblioteca: (nenhuma pasta ativa — use 'Adicionar Pastas' na Web UI)")
	} else {
		fmt.Printf("  Biblioteca: %d pasta(s) somente-leitura ativa(s):\n", len(roots))
		for _, r := range roots {
			fmt.Printf("    - [%s] %s (%s)\n", r.Source, r.Label, r.ID)
		}
	}
	fmt.Println("============================================================")
}

func detectLANIPv4s() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var ips []string
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		ip4 := ipNet.IP.To4()
		if ip4 == nil {
			continue
		}
		s := ip4.String()
		if strings.HasPrefix(s, "169.254.") {
			continue
		}
		ips = append(ips, s)
	}
	return ips
}
