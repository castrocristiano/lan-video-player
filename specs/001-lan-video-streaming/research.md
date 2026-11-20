# Phase 0 Research: Player de Vídeo em Rede Local via Navegador

**Feature**: `001-lan-video-streaming`
**Date**: 2026-11-19
**Status**: Complete

---

## 1. Linguagem e Runtime do Servidor (Binário Único)

- **Decision**: **Go 1.23+** utilizando exclusivamente a biblioteca padrão (`net/http`, `embed`, `os`, `path/filepath`, `io/fs`, `encoding/json`, `log/slog`, `crypto/sha256`).
- **Rationale**:
  - Compila facilmente para um único binário estático (`CGO_ENABLED=0`) sem dependências de bibliotecas C do sistema operacional.
  - `http.ServeContent` na biblioteca padrão do Go implementa nativamente a especificação completa de **HTTP Range Requests (RFC 9110 / RFC 7233)**, incluindo `206 Partial Content`, `Accept-Ranges: bytes`, `Content-Range`, `If-Modified-Since` e `Last-Modified`, além de utilizar chamadas zero-copy (`sendfile`) automaticamente no kernel Linux ao transmitir um `*os.File`.
  - A diretiva `//go:embed` embute todos os arquivos da Web UI diretamente no executável final, cumprindo o Princípio III da Constituição sem exigir ferramentas externas de build.
  - Excelente suporte a concorrência com baixo uso de RAM (< 30 MB ociosos) para múltiplos dispositivos simultâneos na LAN (`SC-004`).
- **Alternatives considered**:
  - *Rust (`axum` + `tokio` + `rust-embed`)*: Excelente performance e segurança de memória, porém exige múltiplas crates externas para HTTP Range streaming, TLS/HTTP server, CLI parsing e serialização, além de tempos de compilação significativamente maiores em containers.
  - *Go com frameworks web externos (`gin`, `echo`, `fiber`)*: Rejeitado pelo Princípio IV (Simplicidade e YAGNI). O roteador padrão `http.ServeMux` do Go 1.22+ suporta padrões de método e path variables (`GET /api/v1/roots/{rootId}/browse`) nativamente sem dependências externas.

---

## 2. Isolamento de Caminhos, Proteção contra Path Traversal e Somente-Leitura

- **Decision**: Camada dedicada `internal/safefs` que canonicaliza cada raiz de mídia na inicialização (`filepath.Abs` + `filepath.EvalSymlinks`) e valida toda requisição de arquivo/pasta com resolução completa de symlinks antes de abrir qualquer descritor somente-leitura (`os.Open`).
- **Rationale**:
  - Atende estritamente ao Princípio II da Constituição e aos requisitos `FR-008`, `FR-009` e `SC-005`.
  - Apenas `filepath.Clean` não impede que um link simbólico criado dentro da pasta de mídia aponte para `/etc/passwd` ou para fora da raiz. Resolver `filepath.EvalSymlinks` no caminho alvo e verificar se o caminho real resultante possui a raiz canonicalizada como prefixo de diretório (`rel, err := filepath.Rel(rootReal, targetReal)` sem iniciar com `..`) bloqueia 100% dos ataques de *path traversal* e fuga por symlink.
  - A interface do pacote `safefs` expõe exclusivamente operações de leitura (`OpenFileReadOnly`, `ReadDir`, `Stat`), tornando impossível a escrita, alteração ou exclusão de arquivos nas pastas de mídia.
- **Alternatives considered**:
  - *Validação puramente lexical de strings (`strings.Contains(path, "..")`)*: Rejeitada por ser vulnerável a codificações alternativas e links simbólicos que apontam para fora da raiz autorizada.

---

## 3. Streaming de Vídeo e Compatibilidade com Navegadores na LAN

- **Decision**: Streaming direto via `http.ServeContent` com detecção explícita de MIME types de vídeo (`.mp4`, `.webm`, `.mkv`, `.mov`, `.m4v`, `.ogv`, `.avi`) e cabeçalhos apropriados (`Accept-Ranges: bytes`, `Cache-Control: no-store` ou validação por `Last-Modified`).
- **Rationale**:
  - Permite início imediato da reprodução (`< 2s`, `SC-003`) e salto instantâneo (*seek*) para qualquer ponto do vídeo sem download completo (`FR-006`).
  - Quando um formato ou codec (ex.: `.mkv` com codec H.265/AC3 em navegador sem suporte) não puder ser decodificado pelo navegador cliente, o elemento HTML5 `<video>` dispara o evento `error` (`MEDIA_ERR_SRC_NOT_SUPPORTED` / `MEDIA_ERR_DECODE`), que a Web UI captura para exibir uma mensagem clara e amigável (`FR-017`, User Story 2 Cenário 4).
- **Alternatives considered**:
  - *Transcodificação HLS/DASH obrigatória em tempo real via FFmpeg*: Rejeitada para a v1 conforme a seção *Assumptions* da especificação e os Princípios III e IV da Constituição (exigiria CPU pesada e quebraria a proposta de binário único leve).

---

## 4. Legendas Locais (`.vtt` e `.srt`) sem Modificar a Biblioteca

- **Decision**: Descoberta automática de arquivos `.vtt` e `.srt` na mesma pasta com o mesmo nome base do vídeo (ex.: `filme.mp4` → `filme.srt`, `filme.pt-BR.srt`, `filme.en.vtt`) e conversão em memória (*on-the-fly*) de `.srt` para `WebVTT` (`text/vtt; charset=utf-8`) ao servir o endpoint de legenda.
- **Rationale**:
  - O elemento nativo `<track kind="subtitles">` do HTML5 exige o formato WebVTT (`.vtt`), enquanto a maioria das bibliotecas domésticas possui legendas em SubRip (`.srt`).
  - Converter `.srt` para `.vtt` em tempo de leitura HTTP (adicionando o cabeçalho `WEBVTT` e convertendo timestamps `00:01:02,345` para `00:01:02.345` em buffer de memória) oferece suporte transparente a `.srt` e `.vtt` sem jamais criar ou modificar arquivos nas pastas de mídia do usuário (`FR-008`, `FR-014`).
- **Alternatives considered**:
  - *Gravar arquivos `.vtt` convertidos ao lado do vídeo*: Rejeitado por violar diretamente o Princípio II da Constituição (acesso estritamente somente-leitura).
  - *Suportar apenas `.vtt`*: Rejeitado por prejudicar a usabilidade em bibliotecas reais onde `.srt` é o padrão predominante.

---

## 5. Geração e Exibição de Miniaturas (Thumbnails) sem Escrita na Mídia

- **Decision**: Estratégia híbrida em três camadas sem escrita nas pastas de mídia:
  1. **Sidecar somente-leitura**: Se já existir uma imagem com o mesmo nome base do vídeo na pasta (`<video>.jpg`, `<video>.jpeg`, `<video>.png`, `<video>.webp`), o servidor a serve em modo somente-leitura.
  2. **Extração sob demanda no servidor (quando `ffmpeg` estiver disponível no PATH/container) em diretório temporário fora da mídia**: O endpoint `GET /api/v1/thumbnails/{rootId}/{path}` extrai 1 frame JPEG redimensionado (`320px` de largura) no instante `5s` (ou `0s` para vídeos curtos) e salva apenas no diretório de cache efêmero do sistema (`/tmp/lan-video-player-thumbs` ou `--cache-dir`), indexado por hash SHA-256 de `(rootId, relPath, size, mtime)`.
  3. **Fallback no cliente e Placeholder SVG**: Se o servidor não tiver `ffmpeg` instalado (retornando `204 No Content`), a Web UI tenta capturar um frame via `<video preload="metadata">` + `<canvas>` no navegador (armazenando o data-URL em `IndexedDB`/cache local) ou exibe imediatamente um cartão SVG padrão limpo e responsivo (`FR-005`).
- **Rationale**:
  - Garante que a listagem de pastas nunca bloqueie (`< 1s` para 1.000 vídeos, `SC-002`), pois as miniaturas são carregadas de forma assíncrona (`loading="lazy"`).
  - Funciona tanto no binário puro em host sem dependências (via fallback do navegador + SVG) quanto no container Podman oficial (que pode incluir `ffmpeg` leve na imagem Alpine) sem jamais tocar nas pastas de mídia (`FR-008`).
- **Alternatives considered**:
  - *Gerar miniaturas sincronamente durante a listagem da pasta*: Rejeitado porque violaria `SC-002` (< 1s para listar 1.000 vídeos).
  - *Exigir `ffmpeg` como dependência obrigatória para o binário iniciar*: Rejeitado por violar o Princípio III (Binário Único autossuficiente).

---

## 6. Web UI Offline, Progresso de Reprodução e Responsividade (Desktop / Mobile / TV)

- **Decision**: Single-Page Application (SPA) leve em HTML5 + CSS Grid/Flexbox + Vanilla JavaScript (ES2022) servida a partir do `embed.FS` do binário Go, utilizando `localStorage` para persistir o progresso de reprodução e preferências locais (velocidade, autoplay, ordenação).
- **Rationale**:
  - 100% offline (`FR-010`, `SC-006`): zero chamadas a Google Fonts, CDNs de ícones ou bibliotecas externas; todos os ícones são SVGs inline/locais.
  - `localStorage` chaveado por identificador estável do vídeo (`lvp:progress:{rootId}:{relPath}`) armazena `{ currentTime, duration, completed, updatedAt }` no navegador de cada dispositivo (`FR-012`), evitando conflitos quando 5+ dispositivos assistem simultaneamente na LAN (`SC-004`) e mantendo precisão de retomada `< 5s` (`SC-007`).
  - Controles amplos com suporte a teclado/controle remoto de TV (setas, `Space`/`Enter`, `F` para tela cheia) e toque em smartphones (`FR-007`, `SC-008`).
- **Alternatives considered**:
  - *SPA com React/Vue/Tailwind e pipeline Node.js*: Rejeitado pelo Princípio IV (Simplicidade e YAGNI) — adiciona centenas de pacotes npm, complexidade de build e auditoria de licença AGPL desnecessária para uma interface enxuta de biblioteca e player.

---

## 7. Empacotamento Container OCI e `podman compose` Rootless

- **Decision**: `Containerfile` multi-stage (estágio 1: `golang:1.23-alpine` compilando binário estático; estágio 2: `alpine:3.20` minimalista com `ffmpeg` opcional e usuário não-root `10001:10001`) + `compose.yaml` configurado para **Podman rootless** e Docker Compose.
- **Rationale**:
  - Cumpre a restrição técnica constitucional de execução com `podman compose` / `podman-compose`.
  - No Podman rootless em distribuições com SELinux (ex.: Fedora/RHEL), volumes de host montados com `:ro,Z` (ou `:ro,z` quando compartilhados) permitem leitura segura pelo processo não-root dentro do container sem permitir qualquer escrita no diretório de mídia do host.
  - O diretório de cache de miniaturas dentro do container usa `tmpfs` ou `/tmp` gravável pelo usuário não-root, mantendo o filesystem raiz do container imutável (`read_only: true`).
- **Alternatives considered**:
  - *Executar container como `root`*: Rejeitado diretamente pela Constituição ("executar o processo como usuário não-root").

---

## 8. Seletor Visual de Pastas na Web UI e Persistência Segura da Biblioteca (`FR-018`, `FR-019`)

- **Decision**:
  1. **Sandbox de Navegação (`--browse-base` / `LVP_BROWSE_BASE`)**: O servidor define um diretório-base seguro (padrão: diretório `$HOME` do usuário no host direto ou `/media` dentro do container). O seletor visual de pastas da Web UI (`GET /api/v1/fs/dirs?path=...`) só pode listar subdiretórios contidos estritamente dentro desse `--browse-base` (validado pela mesma camada `internal/safefs`), impedindo que clientes na LAN naveguem por `/etc`, `/var` ou fora da árvore permitida.
  2. **Seleção Múltipla e Desvinculação (`POST /api/v1/roots` e `DELETE /api/v1/roots/{rootId}`)**: O modal "Gerenciar Biblioteca / Adicionar Pastas" na Web UI permite navegar pela árvore do `--browse-base`, marcar uma ou mais pastas simultaneamente (checkboxes) e adicioná-las como `MediaRoot` ativas em tempo real, além de permitir desvincular pastas adicionadas pela UI sem jamais apagar arquivos do disco.
  3. **Persistência Atômica Fora da Mídia (`--config-file` / `LVP_CONFIG_FILE`)**: As pastas adicionadas pela Web UI são salvas automaticamente em um arquivo JSON leve (`~/.config/lan-video-player/library.json` no host ou `/config/library.json` em volume dedicado no container) via escrita atômica (arquivo temporário + `os.Rename` no mesmo diretório de config). Na inicialização, o servidor combina as pastas de `--media` (somente-leitura fixas de CLI/Env) com as pastas persistidas em `library.json`.
- **Rationale**:
  - Atende ao requisito do usuário de escolher uma ou mais pastas diretamente pela interface web e mantê-las salvas entre reinícios, sem violar o Princípio II da Constituição (as pastas de mídia permanecem 100% intocadas e somente-leitura, e o acesso via rede permanece confinado ao `--browse-base` autorizado).
- **Alternatives considered**:
  - *Permitir navegar livremente por `/` (raiz de todo o sistema operacional) pela Web UI na LAN*: Rejeitado por violar o Princípio II da Constituição ("isolamento do restante do sistema de arquivos").
  - *Salvar o arquivo de configuração dentro da pasta de mídia*: Rejeitado por violar a garantia estrita de zero escrita nas pastas de mídia (`FR-008`).
