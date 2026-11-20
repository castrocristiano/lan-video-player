# Implementation Plan: Player de Vídeo em Rede Local via Navegador

**Branch**: `001-lan-video-streaming` | **Date**: 2026-11-19 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/001-lan-video-streaming/spec.md`

---

## Summary

Construir o `lan-video-player`: um servidor e player de vídeo para rede local (LAN) distribuído como **binário único em Go** com **Web UI responsiva embutida (`//go:embed`)**, operando 100% offline e em modo **estritamente somente-leitura** sobre uma ou mais pastas de mídia configuradas ou adicionadas pelo usuário. A solução inclui um **seletor visual de pastas na Web UI** (restrito a um diretório-base seguro `--browse-base` como `$HOME` ou `/media`, com seleção múltipla e persistência automática em `library.json` fora das pastas de mídia), navegação hierárquica por pastas/subpastas, busca global por nome, ordenação, miniaturas sem escrita nas pastas de mídia, streaming direto via **HTTP Range Requests (`206 Partial Content`)** com seek instantâneo e controle de velocidade, legendas externas locais (`.vtt` e conversão em memória de `.srt`), troca de faixa de áudio suportada pelo navegador, retomada de reprodução por dispositivo (`localStorage`), próximo vídeo automático e empacotamento OCI minimalista compatível com **`podman compose` rootless**.

---

## Technical Context

**Language/Version**: Go 1.23+ (biblioteca padrão `net/http`, `embed`, `os`, `path/filepath`, `io/fs`, `encoding/json`, `log/slog`) + HTML5 / CSS3 / Vanilla JavaScript (ES2022) embutidos no binário.

**Primary Dependencies**: Zero dependências Go externas obrigatórias para o servidor HTTP e streaming (`stdlib` pura); `ffmpeg` opcional no ambiente/container apenas para extração sob demanda de miniaturas em cache temporário fora das pastas de mídia (com fallback automático no cliente/SVG quando ausente).

**Storage**: Sistema de arquivos local acessado estritamente em modo somente-leitura (`internal/safefs`) para vídeos/legendas e para navegação no diretório-base do seletor de pastas (`--browse-base`); arquivo de configuração JSON local fora da mídia (`~/.config/lan-video-player/library.json` ou `--config-file`) para persistir pastas adicionadas pela Web UI; diretório temporário fora da mídia (`/tmp/lan-video-player-thumbs`) para cache efêmero de miniaturas; `localStorage` do navegador de cada dispositivo para progresso de reprodução e preferências.

**Testing**: `go test -v -race ./...` (testes unitários e de integração HTTP cobrindo segurança de caminhos/symlinks, somente-leitura, listagem, busca, streaming `206 Partial Content`, conversão `.srt` → `.vtt` e miniaturas).

**Target Platform**: Linux (x86_64 / ARM64, incluindo Raspberry Pi e servidores domésticos), macOS e Windows como binário único, mais imagem de container OCI compatível com **Podman rootless** (`podman compose` / `podman-compose`) e Docker Compose.

**Project Type**: `web-service` / `cli` (binário único autossuficiente com Web UI SPA embutida).

**Performance Goals**:
- Inicialização do servidor e prontidão na LAN em `< 1s` (`SC-001`).
- Listagem e navegação de pastas em `< 1s` para bibliotecas com até 1.000 vídeos (`SC-002`).
- Início de reprodução e *seek* em `< 2s` via HTTP Range Requests na LAN (`SC-003`).
- Suporte fluido a `>= 5` dispositivos simultâneos transmitindo vídeos distintos na LAN (`SC-004`).

**Constraints**:
- **100% Offline / Zero Cloud**: 0 chamadas externas, 0 CDNs, `Content-Security-Policy: default-src 'self'` (`SC-006`).
- **Somente-Leitura Estrito**: 0 bytes criados, modificados ou removidos dentro das pastas de mídia configuradas (`SC-005`).
- **Baixo consumo de recursos**: `< 30 MB` de RAM em repouso para o servidor Go.
- **Licença**: GNU AGPL v3.0 com arquivo `LICENSE` embutido e acessível na interface web (`FR-016`).

**Scale/Scope**: Bibliotecas locais domésticas ou de pequenas equipes (`1` a `10.000+` vídeos distribuídos em múltiplas pastas raiz), atendendo `1` a `20` espectadores simultâneos na LAN sem banco de dados externo.

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Princípio / Restrição Constitucional | Status Pré-Pesquisa | Validação Pós-Design (Phase 1) |
|--------------------------------------|---------------------|--------------------------------|
| **I. Local-First e Zero Cloud (INEGOCIÁVEL)** | PASS | **PASS** — Todos os arquivos HTML, CSS, JS e ícones SVG são embutidos no binário via `//go:embed`. Cabeçalho `Content-Security-Policy: default-src 'self'` bloqueia qualquer acesso externo. |
| **II. Segurança e Acesso Somente-Leitura à Mídia (INEGOCIÁVEL)** | PASS | **PASS** — O pacote `internal/safefs` resolve `filepath.Abs` + `filepath.EvalSymlinks` nas raízes e em cada requisição, bloqueando `..` e symlinks externos (`403 Forbidden`), expondo apenas leitura (`os.Open`). Miniaturas usam `/tmp` fora da mídia e progresso usa `localStorage` no cliente. |
| **III. Binário Único e Web UI Leve** | PASS | **PASS** — Binário único compilado em Go com `CGO_ENABLED=0` e `embed.FS`, sem banco de dados externo nem dependências de runtime obrigatórias. |
| **IV. Simplicidade e YAGNI** | PASS | **PASS** — Usa `net/http` padrão do Go 1.23+ e Vanilla JS sem frameworks pesados ou pipelines de build complexos. Escopo focado nos requisitos `FR-001` a `FR-017`. |
| **V. Conformidade Aberta (GNU AGPL v3.0)** | PASS | **PASS** — `stdlib` do Go (BSD-3-Clause) é totalmente compatível com GNU AGPL v3.0; `LICENSE` é servido na rota `/LICENSE` e referenciado no rodapé da Web UI (`FR-016`). |
| **Execução em container (`podman compose` rootless)** | PASS | **PASS** — `Containerfile` multi-stage executa como usuário não-root (`10001:10001`), e `compose.yaml` monta volumes de mídia com `:ro,Z` compatível com Podman rootless e SELinux. |
| **Política de testes pragmática** | PASS | **PASS** — Suíte `go test -race ./...` planejada para cobrir `safefs` (path traversal + symlinks), `catalog` (listagem/busca/legendas) e `server` (contratos HTTP e Range `206`). |

---

## Project Structure

### Documentation (this feature)

```text
specs/001-lan-video-streaming/
├── plan.md              # Este arquivo (Plano de Implementação)
├── research.md          # Phase 0 output (Decisões arquiteturais e pesquisa)
├── data-model.md        # Phase 1 output (Modelo de entidades do servidor e cliente)
├── quickstart.md        # Phase 1 output (Guia de execução, Podman e validação)
├── contracts/           # Phase 1 output (Contratos de CLI e HTTP API)
│   └── http-api.md
└── tasks.md             # Phase 2 output (a ser gerado por /speckit-tasks)
```

### Source Code (repository root)

```text
cmd/
└── lan-video-player/
    └── main.go                  # Entrypoint CLI, parsing de flags/env, startup e graceful shutdown

internal/
├── config/
│   ├── config.go                # Leitura de flags CLI (--media, --browse-base, --config-file, --addr) e variáveis LVP_*
│   ├── store.go                 # Persistência atômica de library.json fora das pastas de mídia (FR-019)
│   └── config_test.go           # Testes de inicialização, rejeição de pastas inválidas e persistência JSON
├── safefs/
│   ├── safefs.go                # Isolamento de raízes e browse-base somente-leitura, EvalSymlinks e anti-path-traversal
│   └── safefs_test.go           # Testes de segurança contra ../, encoding e symlinks externos (FR-008, FR-009)
├── catalog/
│   ├── catalog.go               # Gestão dinâmica de raízes (add/remove), listagem de pastas/vídeos, breadcrumbs e busca
│   ├── ffmpeg.go                # Descoberta de ffmpeg/ffprobe e inspeção de duração (/api/v1/probe)
│   ├── picker.go                # Listagem segura de diretórios dentro de --browse-base para o seletor visual (FR-018)
│   ├── subtitles.go             # Descoberta de legendas .vtt/.srt e conversão em memória .srt -> WebVTT
│   ├── thumbnail.go             # Resolução de sidecar somente-leitura e extração opcional em cache efêmero
│   └── catalog_test.go          # Testes unitários de varredura, seletor de pastas, busca, ordenação e SRT->VTT
└── server/
    ├── server.go                # Roteador HTTP (http.ServeMux), middlewares CSP/logging e handlers da API (incluindo POST/DELETE roots, GET fs/dirs e GET probe)
    ├── stream.go                # Handler de streaming HTTP Range 206 e transcodificação on-the-fly via ffmpeg
    └── server_test.go           # Testes de integração HTTP (roots, add/remove folders, browse, search, range 206, 403, subtitles)

web/
├── embed.go                     # Diretiva //go:embed static/* e disponibilização do fs.FS
└── static/
    ├── index.html               # Shell semântico responsivo (Desktop, Mobile e Smart TV) + barra de seek + rodapé AGPLv3
    ├── styles.css               # Estilos locais (CSS Grid, tema cinema, tela cheia YouTube-style com auto-hide)
    └── app.js                   # Lógica de navegação, busca, player com seek/transcode/velocidade/legendas/áudio/autoplay e localStorage

.github/workflows/
├── ci.yml                       # Esteira CI (go vet, go test -race, SonarQube Quality Gate, Auto-PR e Release)
└── deploy-wiki.yml              # Sincronização automática da pasta wiki/ com o GitHub Wiki
wiki/                            # Documentação técnica em Markdown para o GitHub Wiki
Containerfile                    # Build multi-stage (golang:1.23-alpine -> alpine:3.20 não-root)
compose.yaml                     # Configuração pronta para podman compose / docker compose rootless (porta 8090, :ro, keep-id)
sonar-project.properties         # Configuração de análise e cobertura Go para o SonarQube
.gitignore                       # Binários compilados, cobertura e scripts/tokens locais
README.md                        # Documentação principal de uso direto, container e CI/CD
```

**Structure Decision**: Estrutura padrão de projeto Go enxuto (`cmd/`, `internal/`, `web/`) na raiz do repositório, permitindo compilação direta (`go build ./cmd/lan-video-player`), execução de todos os testes com um único comando (`go test ./...`) e embutimento limpo dos arquivos estáticos da Web UI (`web/embed.go`) e da licença `LICENSE`.

---

## Complexity Tracking

> **Nenhuma violação à Constituição identificada.** Todos os 5 princípios fundamentais, restrições de container Podman rootless e portões de qualidade foram atendidos sem necessidade de exceções.
