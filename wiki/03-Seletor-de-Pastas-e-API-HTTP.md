# 03 — Seletor Visual de Pastas e API HTTP

## Seletor Visual de Pastas (`--browse-base`)

Pelo botão **"Pastas da Biblioteca"** na interface web, qualquer usuário na LAN pode:

1. Navegar pelos subdiretórios dentro do `--browse-base` seguro configurado no servidor (ex.: `$HOME` no host direto ou `/media` dentro do container).
2. Marcar uma ou mais pastas simultaneamente e adicioná-las à biblioteca ativa.
3. Desvincular pastas adicionadas pela UI sem jamais excluir arquivos do disco.

As pastas adicionadas são salvas atomicamente (arquivo temporário + `os.Rename`) em `library.json` (`--config-file` / `LVP_CONFIG_FILE`), permanecendo disponíveis após reinícios do servidor ou container.

---

## Endpoints da API (`/api/v1`)

| Método | Rota | Descrição |
|--------|------|-----------|
| `GET` | `/api/v1/roots` | Lista as raízes ativas da biblioteca e metadados do servidor. |
| `POST` | `/api/v1/roots` | Adiciona uma ou mais pastas (`{"paths": [...]}`) relativas ao `--browse-base`. |
| `DELETE` | `/api/v1/roots/{rootId}` | Desvincula uma pasta adicionada pela UI (`source: "ui"`). |
| `GET` | `/api/v1/fs/dirs?path=...` | Lista subpastas seguras dentro de `--browse-base` para o seletor visual. |
| `GET` | `/api/v1/browse?root=...&path=...&sort=...` | Lista subpastas e vídeos com breadcrumbs e legendas associadas. |
| `GET` | `/api/v1/search?q=...&root=...&sort=...` | Busca recursiva insensível a maiúsculas/minúsculas em toda a biblioteca. |
| `GET` | `/api/v1/probe/{rootId}/{path...}` | Retorna duração em segundos (`ffprobe`) e se requer transcodificação. |
| `GET` | `/api/v1/stream/{rootId}/{path...}` | Streaming Range `206` ou transcodificação `?transcode=1&start=<sec>`. |
| `GET` | `/api/v1/subtitles/{rootId}/{path...}` | Entrega `.vtt` ou `.srt` convertido para `text/vtt` em memória. |
| `GET` | `/api/v1/thumbnails/{rootId}/{path...}` | Retorna sidecar, extração `ffmpeg` em `/tmp` ou `204 No Content`. |
| `GET` | `/LICENSE` | Texto integral da licença **GNU AGPL v3.0**. |
