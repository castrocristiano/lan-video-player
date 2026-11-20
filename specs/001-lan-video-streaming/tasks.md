# Tasks: Player de Vídeo em Rede Local via Navegador

**Input**: Design documents from `/specs/001-lan-video-streaming/`

**Prerequisites**: [plan.md](plan.md) (required), [spec.md](spec.md) (required for user stories), [research.md](research.md), [data-model.md](data-model.md), [contracts/http-api.md](contracts/http-api.md), [quickstart.md](quickstart.md)

**Tests**: Testes automatizados unitários e de integração são **OBRIGATÓRIOS** pela Constituição do projeto (`.specify/memory/constitution.md`) para segurança de caminhos (`safefs`), varredura/catálogo, persistência da biblioteca e contratos HTTP/Range streaming.

**Organization**: As tarefas estão agrupadas por história de usuário (`US1` a `US4`) para permitir implementação, teste e entrega incremental independente.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Pode ser executada em paralelo (arquivos distintos, sem dependências de tarefas abertas)
- **[Story]**: História de usuário associada (`[US1]`, `[US2]`, `[US3]`, `[US4]`)
- Todas as tarefas incluem caminhos de arquivos exatos no repositório

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Inicialização do módulo Go, estrutura de diretórios do projeto e configuração de empacotamento básico.

- [X] T001 Initialize Go module (`go.mod` targeting Go 1.23+) and directory structure (`cmd/lan-video-player/`, `internal/config/`, `internal/safefs/`, `internal/catalog/`, `internal/server/`, `web/static/`) in `go.mod`
- [X] T002 [P] Configure `.gitignore` for compiled binaries (`bin/`, `lan-video-player`), coverage profiles, and local temporary directories in `.gitignore`
- [X] T003 [P] Create embedded filesystem loader for `web/static/*` and root `LICENSE` file in `web/embed.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Infraestrutura central de segurança somente-leitura (`safefs`), configuração CLI/Env, persistência de `library.json` e esqueleto do servidor HTTP com CSP offline.

**⚠️ CRITICAL**: Nenhuma história de usuário pode ser iniciada antes da conclusão desta fase.

- [X] T004 Implement read-only root & path canonicalization (`filepath.Abs`, `filepath.EvalSymlinks`, symlink escape rejection, hidden file check, and read-only `os.Open` wrappers) in `internal/safefs/safefs.go`
- [X] T005 [P] Write unit tests for path traversal (`../`, encoded segments), symlink escape outside root, and read-only enforcement in `internal/safefs/safefs_test.go`
- [X] T006 Implement CLI flags and environment variables parser (`--media`/`LVP_MEDIA_DIRS`, `--browse-base`/`LVP_BROWSE_BASE`, `--config-file`/`LVP_CONFIG_FILE`, `--addr`/`LVP_ADDR`, `--cache-dir`/`LVP_CACHE_DIR`, `--no-thumbs`/`LVP_NO_THUMBS`, `--version`) in `internal/config/config.go`
- [X] T007 Implement atomic JSON load/save for persistent UI library folders conforming to `data-model.md` (`LibraryConfig.Version`, `LibraryConfig.Roots`) in `internal/config/store.go`
- [X] T008 [P] Write unit tests for startup directory validation (`FR-002`) and atomic `library.json` persistence (`FR-019`) in `internal/config/config_test.go`
- [X] T009 Setup base HTTP server, standard JSON error helper, offline `Content-Security-Policy` middleware (`default-src 'self'`), static `web/embed.go` handler, and `/LICENSE` route in `internal/server/server.go`
- [X] T010 Create application entrypoint wiring `config`, `safefs`, `catalog`, `server`, LAN IP startup banner, and graceful shutdown in `cmd/lan-video-player/main.go`

**Checkpoint**: Fundação pronta — isolamento `safefs`, configuração CLI/JSON e servidor HTTP base operacionais.

---

## Phase 3: User Story 1 - Configurar, Selecionar e Navegar em Uma ou Mais Pastas de Mídia pela Rede Local (Priority: P1) 🎯 MVP

**Goal**: Permitir visualizar raízes configuradas, abrir o seletor visual de pastas (restrito ao `--browse-base` seguro) na Web UI para selecionar e adicionar uma ou mais pastas à biblioteca (com persistência automática em `library.json`) ou desvinculá-las, e navegar por pastas, subpastas e vídeos com trilha de navegação (breadcrumbs) 100% offline na LAN.

**Independent Test**: Iniciar o servidor com um `--browse-base` contendo subpastas e vídeos, abrir a Web UI no navegador, adicionar duas pastas pelo modal "Gerenciar Biblioteca / Adicionar Pastas", confirmar que sobrevivem ao reinício em `library.json`, navegar pelas subpastas via breadcrumbs e verificar que arquivos não-vídeo/ocultos são omitidos e qualquer acesso fora das raízes/base retorna `403 Forbidden`.

### Tests for User Story 1 ⚠️

- [X] T011 [P] [US1] Write unit tests for media root management, directory browsing, breadcrumbs generation, and `--browse-base` visual folder picker in `internal/catalog/catalog_test.go`
- [X] T012 [P] [US1] Write HTTP integration tests for `GET /api/v1/roots`, `GET /api/v1/fs/dirs`, `POST /api/v1/roots`, `DELETE /api/v1/roots/{rootId}`, and `GET /api/v1/browse` (including `403 FORBIDDEN_PATH`) in `internal/server/server_test.go`

### Implementation for User Story 1

- [X] T013 [US1] Implement domain types and root registry (`AddRoots`, `RemoveRoot`, label collision disambiguation, and directory browsing) conforming to `data-model.md` (`MediaRoot.ID`, `MediaRoot.Label`, `MediaRoot.CanonicalPath`, `MediaRoot.RelToBrowseBase`, `MediaRoot.Source`, `MediaRoot.Removable`, `BreadcrumbItem.Label`, `BreadcrumbItem.RootID`, `BreadcrumbItem.RelPath`, `FolderEntry.Name`, `FolderEntry.RootID`, `FolderEntry.RelPath`, `FolderEntry.ModifiedAt`, `VideoItem.ID`, `VideoItem.RootID`, `VideoItem.RootLabel`, `VideoItem.Name`, `VideoItem.Title`, `VideoItem.RelPath`, `VideoItem.FolderRelPath`, `VideoItem.SizeBytes`, `VideoItem.ModifiedAt`, `VideoItem.MimeType`, `VideoItem.StreamURL`, `VideoItem.ThumbnailURL`) in `internal/catalog/catalog.go`
- [X] T014 [US1] Implement `--browse-base` directory listing service for the visual folder picker conforming to `data-model.md` (`PickerDirEntry.Name`, `PickerDirEntry.RelPath`, `PickerDirEntry.HasSubdirs`, `PickerDirEntry.AlreadyAdded`) in `internal/catalog/picker.go`
- [X] T015 [US1] Implement HTTP handlers for `GET /api/v1/roots`, `GET /api/v1/fs/dirs`, `POST /api/v1/roots`, `DELETE /api/v1/roots/{rootId}`, and `GET /api/v1/browse` per `contracts/http-api.md` in `internal/server/server.go`
- [X] T016 [P] [US1] Build semantic offline HTML5 shell with top navigation bar, root switcher, breadcrumbs bar, "Gerenciar Biblioteca / Adicionar Pastas" modal dialog, folder/video grid, empty/error states, and GNU AGPL v3.0 footer (`FR-016`) in `web/static/index.html`
- [X] T017 [P] [US1] Create responsive CSS layout (Desktop, Mobile, Smart TV focus rings, dark cinema theme, folder picker modal, and card grid without horizontal overflow) in `web/static/styles.css`
- [X] T018 [US1] Implement frontend state and API integration for loading roots, browsing folders via breadcrumbs, and multi-folder selection/unlinking in the "Adicionar Pastas" modal in `web/static/app.js`

**Checkpoint**: User Story 1 completa — o usuário pode selecionar/adicionar pastas pela Web UI (com persistência em `library.json`) e navegar por toda a hierarquia de pastas e vídeos na LAN.

---

## Phase 4: User Story 2 - Reproduzir Vídeos no Navegador com Controles Essenciais e Velocidade (Priority: P1) 🎯 MVP

**Goal**: Transmitir vídeos diretamente para o navegador via HTTP Range Requests (`206 Partial Content`) com início rápido (`< 2s`), seek imediato para qualquer instante, controle de velocidade (`0.5x` a `2x`), volume/mudo, tela cheia e tratamento amigável de formatos incompatíveis.

**Independent Test**: Selecionar um vídeo na listagem da Web UI, confirmar início imediato da reprodução, avançar/retroceder para 70% da barra de progresso (`206 Partial Content`), alternar velocidade para `1.5x` e `0.75x`, testar tela cheia e verificar mensagem amigável ao abrir codec/formato não suportado pelo navegador.

### Tests for User Story 2 ⚠️

- [X] T019 [P] [US2] Write HTTP integration tests for `GET /api/v1/stream/{rootId}/{path...}` covering `200 OK`, `206 Partial Content` byte ranges, `416 Range Not Satisfiable`, `403 Forbidden` path traversal, and `404 Not Found` in `internal/server/server_test.go`

### Implementation for User Story 2

- [X] T020 [US2] Implement HTTP Range video streaming handler using `safefs` read-only file descriptors and `http.ServeContent` with explicit video MIME types in `internal/server/stream.go`
- [X] T021 [US2] Register `GET /api/v1/stream/{rootId}/{path...}` route and URL path decoding/validation in `internal/server/server.go`
- [X] T022 [US2] Implement video player view/modal in the Web UI with playback controls (play/pause, seek timeline, volume/mute, fullscreen, keyboard/TV remote shortcuts, playback speed selector `0.5x`–`2x`, and unsupported format error banner `FR-017`) persisting `PlayerPreferences.playbackRate`, `PlayerPreferences.volume`, and `PlayerPreferences.muted` per `data-model.md` in `web/static/app.js`

**Checkpoint**: MVP Funcional Completo (`US1` + `US2`) — seleção/navegação de pastas e reprodução de vídeo com seek e controle de velocidade na LAN.

---

## Phase 5: User Story 3 - Buscar, Ordenar e Identificar Vídeos por Miniaturas (Priority: P2)

**Goal**: Exibir miniaturas de prévia nos cartões de vídeo (sem gravar nada nas pastas de mídia), filtrar vídeos pelo nome em toda a biblioteca e ordenar itens por nome (`A–Z` / `Z–A`) ou data de modificação.

**Independent Test**: Em uma biblioteca com várias pastas e vídeos, verificar o carregamento assíncrono das miniaturas (sidecar, extração em `/tmp` via `ffmpeg` ou fallback no cliente/SVG), buscar por termo parcial sem diferenciar maiúsculas/minúsculas e alternar a ordenação entre Nome e Data.

### Tests for User Story 3 ⚠️

- [X] T023 [P] [US3] Write unit tests for case-insensitive global/root search, sorting criteria (`name_asc`, `name_desc`, `date_desc`, `date_asc`), and read-only thumbnail resolution in `internal/catalog/catalog_test.go`
- [X] T024 [P] [US3] Write HTTP integration tests for `GET /api/v1/search` and `GET /api/v1/thumbnails/{rootId}/{path...}` (verifying zero writes inside media roots) in `internal/server/server_test.go`

### Implementation for User Story 3

- [X] T025 [P] [US3] Implement recursive case-insensitive video search and sorting (`name_asc`, `name_desc`, `date_desc`, `date_asc`) across active media roots in `internal/catalog/catalog.go`
- [X] T026 [P] [US3] Implement read-only sidecar thumbnail lookup and optional on-demand `ffmpeg` frame extraction into ephemeral `--cache-dir` outside media roots (returning `204 No Content` fallback when unavailable) in `internal/catalog/thumbnail.go`
- [X] T027 [US3] Register and implement `GET /api/v1/search` and `GET /api/v1/thumbnails/{rootId}/{path...}` handlers per `contracts/http-api.md` in `internal/server/server.go`
- [X] T028 [US3] Implement debounced search bar, sort selector (persisting `PlayerPreferences.sortBy` per `data-model.md`), lazy thumbnail rendering with client-side `<video>`+`<canvas>` / SVG placeholder fallback, and "nenhum vídeo encontrado" clear-filter state in `web/static/app.js`

**Checkpoint**: User Story 3 completa — descoberta rápida por miniaturas, busca global e ordenação funcionando sobre a biblioteca.

---

## Phase 6: User Story 4 - Retomar Reprodução, Próximo Vídeo Automático, Legendas e Faixas de Áudio (Priority: P3)

**Goal**: Salvar localmente no navegador o progresso de cada vídeo para retomar de onde parou (com barra de progresso nos cartões e reset automático ao concluir `>= 95%`), avançar manual ou automaticamente para o próximo vídeo da pasta (com contagem regressiva cancelável), carregar legendas locais (`.vtt` e `.srt` convertido em memória) e permitir alternar faixas de áudio expostas pelo navegador.

**Independent Test**: Assistir parcialmente a um vídeo e recarregar a página para validar o indicador visual e a retomada (`± 5s`); concluir um vídeo para validar a contagem regressiva de próximo vídeo automático; ativar uma legenda `.srt` local no player e verificar o seletor de faixa de áudio.

### Tests for User Story 4 ⚠️

- [X] T029 [P] [US4] Write unit tests for local subtitle discovery (`.vtt`, `.srt`, language suffix parsing) and in-memory `.srt` to `WebVTT` conversion in `internal/catalog/catalog_test.go`
- [X] T030 [P] [US4] Write HTTP integration tests for `GET /api/v1/subtitles/{rootId}/{path...}` verifying `text/vtt; charset=utf-8` output for both `.vtt` and `.srt` files without disk modification in `internal/server/server_test.go`

### Implementation for User Story 4

- [X] T031 [US4] Implement sidecar subtitle discovery and in-memory `.srt` → `WebVTT` converter conforming to `data-model.md` (`VideoItem.Subtitles`, `SubtitleTrack.ID`, `SubtitleTrack.Label`, `SubtitleTrack.Lang`, `SubtitleTrack.Format`, `SubtitleTrack.RelPath`, `SubtitleTrack.SrcURL`) in `internal/catalog/subtitles.go`
- [X] T032 [US4] Implement `GET /api/v1/subtitles/{rootId}/{path...}` handler in `internal/server/server.go`
- [X] T033 [US4] Implement device-local playback progress tracking in `localStorage` (`lvp:progress:{rootId}:{relPath}`), card progress bar indicator, resume prompt/action, and `>= 95%` completion reset conforming to `data-model.md` (`PlaybackProgress.videoId`, `PlaybackProgress.currentTime`, `PlaybackProgress.duration`, `PlaybackProgress.progressRatio`, `PlaybackProgress.completed`, `PlaybackProgress.updatedAt`) in `web/static/app.js`
- [X] T034 [US4] Implement previous/next folder video navigation, toggleable auto-play next countdown overlay (persisting `PlayerPreferences.autoPlayNext` per `data-model.md`), `<track kind="subtitles">` selector, and multi-audio track selector (`video.audioTracks` with single-track graceful fallback) in `web/static/app.js`

**Checkpoint**: Todas as 4 histórias de usuário (`US1`–`US4`) implementadas e testáveis de ponta a ponta.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Empacotamento OCI para **Podman rootless** (`Containerfile` + `compose.yaml`), documentação final e validação completa dos portões de qualidade da Constituição.

- [X] T035 [P] Create multi-stage `Containerfile` (`golang:1.23-alpine` static build → `alpine:3.20` runtime with optional `ffmpeg` and non-root user `10001:10001`) in `Containerfile`
- [X] T036 [P] Create `compose.yaml` compatible with rootless `podman compose` / `podman-compose` and `docker compose` (read-only media volume mounts with `:ro,Z`, persistent `/config` volume for `library.json`, and ephemeral `tmpfs` for `/tmp`) in `compose.yaml`
- [X] T037 [P] Write project documentation covering binary execution, Web UI folder selection, Podman rootless setup (`:ro,Z`), CLI/env flags, and GNU AGPL v3.0 license notice in `README.md`
- [X] T038 Execute full test suite (`go test -v -race ./...`), static binary build verification, and end-to-end validation checks from `specs/001-lan-video-streaming/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1: Setup** (`T001`–`T003`): Sem dependências — inicia imediatamente.
- **Phase 2: Foundational** (`T004`–`T010`): Depende da Phase 1 — **BLOQUEIA** todas as histórias de usuário.
- **Phase 3: User Story 1 (P1)** (`T011`–`T018`): Depende da Phase 2.
- **Phase 4: User Story 2 (P1)** (`T019`–`T022`): Depende da Phase 2 (integra-se à listagem da `US1` para compor o MVP completo).
- **Phase 5: User Story 3 (P2)** (`T023`–`T028`): Depende da Phase 3 (`US1`).
- **Phase 6: User Story 4 (P3)** (`T029`–`T034`): Depende da Phase 4 (`US2`).
- **Phase 7: Polish & Podman Packaging** (`T035`–`T038`): Depende da conclusão das histórias desejadas.

### User Story Dependency Graph

```text
Phase 1: Setup
   │
   ▼
Phase 2: Foundational (safefs + config/store + base server)
   │
   ├──────────────────────────────┐
   ▼                              ▼
Phase 3: US1 (P1)              Phase 4: US2 (P1)
[Pastas, Seletor UI & Nav]     [Streaming Range 206 & Player]
   │                              │
   ▼                              ▼
Phase 5: US3 (P2)              Phase 6: US4 (P3)
[Busca, Ordenação & Thumbs]    [Retomada, Autoplay, Legendas & Áudio]
   │                              │
   └──────────────┬───────────────┘
                  ▼
       Phase 7: Polish & Podman
```

### Parallel Opportunities

- **Setup**: `T002` (`.gitignore`) e `T003` (`web/embed.go`) podem rodar em paralelo após `T001`.
- **Foundational**: `T005` (`safefs_test.go`) e `T008` (`config_test.go`) podem rodar em paralelo com os demais módulos da fundação.
- **User Story 1**: `T011` e `T012` (testes) em paralelo; `T016` (`index.html`) e `T017` (`styles.css`) em paralelo com o backend (`T013`–`T015`).
- **User Story 3**: `T025` (busca/ordenação em `catalog.go`) e `T026` (miniaturas em `thumbnail.go`) atuam em arquivos distintos e podem rodar em paralelo.
- **Polish**: `T035` (`Containerfile`), `T036` (`compose.yaml`) e `T037` (`README.md`) podem ser executadas todas em paralelo.

---

## Parallel Example: User Story 1 & User Story 3

```bash
# User Story 1 - Construir HTML e CSS da Web UI em paralelo com os serviços Go:
Task T016: "Build semantic offline HTML5 shell in web/static/index.html"
Task T017: "Create responsive CSS layout in web/static/styles.css"

# User Story 3 - Implementar busca e serviço de miniaturas em paralelo:
Task T025: "Implement recursive case-insensitive video search and sorting in internal/catalog/catalog.go"
Task T026: "Implement read-only sidecar thumbnail lookup and optional ffmpeg extraction in internal/catalog/thumbnail.go"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2 — P1)

1. Concluir **Phase 1: Setup** (`T001`–`T003`).
2. Concluir **Phase 2: Foundational** (`T004`–`T010`) — garantindo 100% dos testes de segurança `safefs` e persistência `library.json`.
3. Concluir **Phase 3: User Story 1** (`T011`–`T018`) — seletor visual de pastas na Web UI + navegação hierárquica.
4. Concluir **Phase 4: User Story 2** (`T019`–`T022`) — streaming HTTP Range `206` e player com seek e velocidade.
5. **VALIDAR MVP**: Testar seleção de pastas, navegação e reprodução de vídeo na LAN.

### Incremental Delivery (P2 → P3 → Container Podman)

1. Adicionar **Phase 5: User Story 3 (P2)** (`T023`–`T028`) → Busca global, ordenação e miniaturas sem escrita na mídia.
2. Adicionar **Phase 6: User Story 4 (P3)** (`T029`–`T034`) → Retomada de progresso (`localStorage`), próximo vídeo automático, legendas `.srt`/`.vtt` e troca de faixa de áudio.
3. Finalizar **Phase 7: Polish** (`T035`–`T038`) → `Containerfile`, `compose.yaml` para Podman rootless (`:ro,Z`), `README.md` e validação completa.
