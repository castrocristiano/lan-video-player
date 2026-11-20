# lan-video-player

Servidor e player de vídeo leve para rede local (**LAN**), distribuído como **binário único em Go** com **Web UI embutida (`//go:embed`)**, operando **100% offline** e em modo **estritamente somente-leitura** sobre suas pastas de mídia.

---

## Principais Funcionalidades

- **100% Offline & Zero Cloud**: Nenhum CDN externo, telemetria ou dependência de internet (`Content-Security-Policy: default-src 'self'`).
- **Somente-Leitura Estrito & Proteção de Caminhos**: Isolamento via `filepath.Abs` + `filepath.EvalSymlinks` (`internal/safefs`), bloqueando *path traversal* (`..`) e links simbólicos externos. Nenhum arquivo é criado ou modificado nas pastas de mídia.
- **Seletor Visual de Pastas na Web UI**: Adicione ou desvincule uma ou mais pastas diretamente pelo navegador (dentro do diretório-base autorizado `--browse-base`), com salvamento automático em `library.json` (fora das pastas de mídia).
- **Streaming HTTP Range (`206 Partial Content`)**: Início imediato e *seek* instantâneo na linha do tempo sem precisar baixar o arquivo completo.
- **Controles Completos**: Velocidade de reprodução (`0.5x` a `2.0x`), tela cheia, atalhos de teclado/controle de TV, retomada automática de onde parou por dispositivo (`localStorage`), próximo vídeo automático com contagem regressiva, legendas locais (`.vtt` e conversão em memória de `.srt`) e troca de faixa de áudio.
- **Pronto para Podman Rootless**: `Containerfile` multi-stage não-root (`10001:10001`) e `compose.yaml` compatível com `podman compose` (`:ro,Z` para SELinux).

---

## 1. Compilação e Execução Direta (Binário Único)

### Compilar

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/lan-video-player ./cmd/lan-video-player
```

### Executar

```bash
./bin/lan-video-player \
  --addr 0.0.0.0:8080 \
  --browse-base "$HOME/Videos" \
  --media "$HOME/Videos/Filmes" \
  --media "$HOME/Videos/Series"
```

Ou inicie apenas com `--browse-base "$HOME/Videos"` e selecione as pastas diretamente no botão **"Pastas da Biblioteca"** na interface web (`http://localhost:8080` ou `http://<IP-DA-LAN>:8080`).

---

## 2. Execução com `podman compose` (Rootless)

```bash
mkdir -p ./media
LVP_HOST_MEDIA=/caminho/para/seus/videos LVP_PORT=8080 podman compose up -d --build
```

*(Também compatível com `podman-compose up -d --build` e `docker compose up -d --build`.)*

---

## 3. Flags de Linha de Comando e Variáveis de Ambiente

| Flag CLI | Variável de Ambiente | Padrão | Descrição |
|----------|----------------------|--------|-----------|
| `--media <path>` (repetível) | `LVP_MEDIA_DIRS` | (opcional) | Pastas raiz de mídia iniciais (somente-leitura). |
| `--browse-base <path>` | `LVP_BROWSE_BASE` | `$HOME` (ou `/media` no container) | Diretório-base seguro para o seletor visual de pastas na Web UI. |
| `--config-file <path>` | `LVP_CONFIG_FILE` | `~/.config/lan-video-player/library.json` | Arquivo JSON local (fora da mídia) onde as pastas adicionadas na UI são salvas. |
| `--addr <host:port>` | `LVP_ADDR` | `0.0.0.0:8080` | Endereço e porta HTTP na LAN. |
| `--cache-dir <path>` | `LVP_CACHE_DIR` | `/tmp/lan-video-player-thumbs` | Cache temporário fora da mídia para miniaturas extraídas via `ffmpeg`. |
| `--no-thumbs` | `LVP_NO_THUMBS` | `false` | Desativa extração de miniaturas no servidor via `ffmpeg`. |
| `--version` | - | - | Exibe a versão e informações de licença. |

---

## 4. Testes Automatizados

```bash
go test -v -race ./...
```

---

## Licença

Este projeto é software livre licenciado sob a **GNU Affero General Public License v3.0 (GNU AGPL v3.0)**. Consulte o arquivo [LICENSE](LICENSE) ou acesse `/LICENSE` na interface web do servidor em execução.
