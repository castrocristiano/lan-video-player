# 01 — Arquitetura e Segurança Somente-Leitura

## Visão Geral dos Pacotes (Go)

O servidor é estruturado em pacotes enxutos sem dependências externas obrigatórias (`stdlib` pura):

```text
cmd/lan-video-player/     # Entrypoint CLI, banner com IPs da LAN e graceful shutdown
internal/config/          # Parsing de flags/env (LVP_*) e persistência atômica (library.json)
internal/safefs/          # Isolamento de raízes, EvalSymlinks e abertura O_RDONLY
internal/catalog/         # Raízes, listagem, busca, ordenação, legendas, miniaturas e FFmpeg probe
internal/server/          # Roteador HTTP, CSP offline, endpoints REST e Range streaming
web/                      # Assets da SPA (index.html, styles.css, app.js, LICENSE) via //go:embed
```

---

## Isolamento de Caminhos (`internal/safefs`)

Toda operação de leitura de pastas, vídeos, legendas e miniaturas passa obrigatoriamente por `safefs.Root`:

1. **Canonicalização na inicialização**: Cada raiz configurada (`--media` ou adicionada pela UI) e o `--browse-base` são resolvidos com `filepath.Abs` + `filepath.EvalSymlinks`.
2. **Bloqueio de *Path Traversal* e Symlinks Externos**:
   - Segmentos `..`, bytes nulos (`\x00`), barras invertidas e arquivos/pastas ocultos (iniciados por `.`) são rejeitados com `403 Forbidden` (`FORBIDDEN_PATH`).
   - Qualquer link simbólico cujo destino real aponte para fora da raiz autorizada é bloqueado antes de abrir o descritor de arquivo.
3. **Garantia Estrita de Somente-Leitura**:
   - Arquivos são abertos exclusivamente com `os.OpenFile(..., os.O_RDONLY, 0)`.
   - Miniaturas extraídas ficam em `/tmp/lan-video-player-thumbs` (fora da mídia), a configuração da biblioteca fica em `/config/library.json` (fora da mídia) e o progresso de cada usuário fica no `localStorage` do navegador.
