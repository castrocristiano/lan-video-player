# Interface Contracts: CLI & HTTP API (`lan-video-player`)

**Feature**: `001-lan-video-streaming`
**Date**: 2026-11-19

---

## 1. Contrato de Linha de Comando (CLI) e Variáveis de Ambiente

O binário `lan-video-player` DEVE aceitar configuração via flags de linha de comando e variáveis de ambiente (`FR-001`, `FR-002`):

| Flag CLI | Variável de Ambiente | Padrão | Descrição |
|----------|----------------------|--------|-----------|
| `--media <path>` (repetível) | `LVP_MEDIA_DIRS` (separado por `:` no Linux/macOS ou `;` no Windows) | (opcional se `library.json` ou `--browse-base` estiver configurado; `.` se nenhum for informado) | Uma ou mais pastas raiz de mídia iniciais acessadas em modo estritamente somente-leitura. |
| `--browse-base <path>` | `LVP_BROWSE_BASE` | `$HOME` (no host direto) ou `/media` (no container) | Diretório-base seguro dentro do qual o seletor visual da Web UI pode navegar para permitir que o usuário selecione e adicione uma ou mais pastas à biblioteca (`FR-018`). |
| `--config-file <path>` | `LVP_CONFIG_FILE` | `~/.config/lan-video-player/library.json` | Arquivo JSON local (fora das pastas de mídia) onde as pastas adicionadas pela Web UI são salvas automaticamente para persistir entre reinícios (`FR-019`). |
| `--addr <host:port>` | `LVP_ADDR` | `0.0.0.0:8080` | Endereço e porta TCP para escuta HTTP na rede local. |
| `--cache-dir <path>` | `LVP_CACHE_DIR` | `<os.TempDir()>/lan-video-player-thumbs` | Diretório efêmero fora das pastas de mídia para cache de miniaturas extraídas sob demanda. |
| `--no-thumbs` | `LVP_NO_THUMBS` | `false` | Desativa extração de miniaturas no servidor via `ffmpeg`. |
| `--version` | - | - | Exibe versão do binário e licença (`GNU AGPL v3.0`) e encerra com código `0`. |

### Comportamento de Inicialização (`FR-002`)
- Se qualquer diretório configurado em `--media` / `LVP_MEDIA_DIRS` não existir, não for um diretório ou não puder ser aberto para leitura, o processo DEVE registrar mensagem clara no `stderr` indicando o caminho inválido e encerrar com código de saída diferente de zero (`exit 1`).
- Ao iniciar com sucesso, o servidor DEVE imprimir no `stdout` as raízes carregadas e as URLs locais (`http://<lan-ip>:<port>`) para acesso imediato (`SC-001`).

---

## 2. Contrato da API HTTP

Todas as rotas da API respondem sob o prefixo `/api/v1` e nunca realizam escrita nas pastas de mídia (`FR-008`).

### Formato Padrão de Erro JSON

```json
{
  "error": {
    "code": "NOT_FOUND | FORBIDDEN_PATH | INVALID_REQUEST | INTERNAL_ERROR",
    "message": "Mensagem amigável em português explicando o problema."
  }
}
```

---

### 2.1 `GET /api/v1/roots`

Lista as raízes de mídia ativas na biblioteca (combinando as pastas de inicialização e as pastas adicionadas pela Web UI) e metadados do servidor (incluindo suporte ao seletor de pastas e informações de licença para `FR-016`, `FR-018`).

- **Response `200 OK`** (`application/json`):
```json
{
  "roots": [
    {
      "id": "root-1",
      "label": "Filmes",
      "relToBrowseBase": "Videos/Filmes",
      "source": "cli",
      "removable": false
    },
    {
      "id": "root-2",
      "label": "Séries",
      "relToBrowseBase": "Videos/Series",
      "source": "ui",
      "removable": true
    }
  ],
  "browseBase": {
    "enabled": true,
    "label": "Diretório Base"
  },
  "meta": {
    "appName": "lan-video-player",
    "version": "0.1.0",
    "license": "GNU AGPL v3.0",
    "sourceUrl": "/LICENSE"
  }
}
```

---

### 2.1.1 `GET /api/v1/fs/dirs`

Lista subdiretórios dentro do `--browse-base` seguro para alimentar o seletor visual de pastas ("Adicionar Pastas à Biblioteca") na Web UI (`FR-018`).

- **Query Parameters**:
  - `path` (opcional, padrão `""`): Caminho relativo dentro do `--browse-base` (ex.: `Videos`).
- **Response `200 OK`** (`application/json`):
```json
{
  "baseLabel": "Diretório Base",
  "currentPath": "Videos",
  "breadcrumbs": [
    { "label": "Diretório Base", "relPath": "" },
    { "label": "Videos", "relPath": "Videos" }
  ],
  "directories": [
    {
      "name": "Filmes",
      "relPath": "Videos/Filmes",
      "hasSubdirs": true,
      "alreadyAdded": true
    },
    {
      "name": "Cursos",
      "relPath": "Videos/Cursos",
      "hasSubdirs": true,
      "alreadyAdded": false
    },
    {
      "name": "Documentarios",
      "relPath": "Videos/Documentarios",
      "hasSubdirs": false,
      "alreadyAdded": false
    }
  ]
}
```
- **Error Responses**:
  - `403 Forbidden`: Tentativa de navegar fora do `--browse-base` (`FORBIDDEN_PATH`).
  - `404 Not Found`: Diretório relativo não encontrado.

---

### 2.1.2 `POST /api/v1/roots`

Adiciona uma ou mais pastas selecionadas no seletor visual (relativas ao `--browse-base`) à biblioteca ativa e persiste automaticamente no arquivo `library.json` (`FR-018`, `FR-019`).

- **Request Body** (`application/json`):
```json
{
  "paths": [
    "Videos/Cursos",
    "Videos/Documentarios"
  ]
}
```
- **Behavior**:
  - Cada caminho em `paths` é validado pela camada `safefs` contra o `--browse-base` (bloqueando `..` e symlinks externos).
  - Pastas que já estiverem registradas na biblioteca são ignoradas de forma idempotente sem gerar duplicatas.
  - A nova lista de pastas `source: "ui"` é salva atomicamente em `--config-file` (`library.json`, fora das pastas de mídia).
- **Response `200 OK`** (`application/json`): Retorna a lista atualizada de `roots` (mesmo formato de `GET /api/v1/roots`).
- **Error Responses**:
  - `400 Bad Request`: Lista `paths` vazia ou caminho que não é um diretório.
  - `403 Forbidden`: Caminho aponta para fora do `--browse-base` (`FORBIDDEN_PATH`).
  - `404 Not Found`: Pasta selecionada não existe no disco.

---

### 2.1.3 `DELETE /api/v1/roots/{rootId}`

Remove (desvincula) uma pasta adicionada pela Web UI (`source: "ui"`) da biblioteca ativa e atualiza `library.json` (`FR-018`, `FR-019`). **Jamais exclui ou altera arquivos no disco.**

- **Response `200 OK`** (`application/json`): Retorna a lista atualizada de `roots`.
- **Error Responses**:
  - `400 Bad Request`: Tentativa de remover uma raiz fixa de inicialização (`source: "cli"`, `removable: false`).
  - `404 Not Found`: `rootId` não encontrado.

---

### 2.2 `GET /api/v1/browse`

Lista subpastas e vídeos de uma pasta específica dentro de uma raiz configurada (`FR-003`, `FR-004`, `FR-005`).

- **Query Parameters**:
  - `root` (obrigatório quando houver múltiplas raízes; opcional quando houver apenas 1 raiz, assumindo `root-1`): ID da `MediaRoot`.
  - `path` (opcional, padrão `""`): Caminho relativo dentro da raiz (ex.: `Temporada 1`).
  - `sort` (opcional, padrão `name_asc`): `name_asc` | `name_desc` | `date_desc` | `date_asc`.

- **Response `200 OK`** (`application/json`):
```json
{
  "root": {
    "id": "root-1",
    "label": "Séries"
  },
  "currentPath": "Temporada 1",
  "breadcrumbs": [
    { "label": "Séries", "rootId": "root-1", "relPath": "" },
    { "label": "Temporada 1", "rootId": "root-1", "relPath": "Temporada 1" }
  ],
  "folders": [
    {
      "name": "Extras",
      "rootId": "root-1",
      "relPath": "Temporada 1/Extras",
      "modifiedAt": "2026-11-19T18:00:00Z"
    }
  ],
  "videos": [
    {
      "id": "root-1:Temporada 1/Episodio 01.mp4",
      "rootId": "root-1",
      "rootLabel": "Séries",
      "name": "Episodio 01.mp4",
      "title": "Episodio 01",
      "relPath": "Temporada 1/Episodio 01.mp4",
      "folderRelPath": "Temporada 1",
      "sizeBytes": 157286400,
      "modifiedAt": "2026-11-19T18:00:00Z",
      "mimeType": "video/mp4",
      "streamUrl": "/api/v1/stream/root-1/Temporada%201/Episodio%2001.mp4",
      "thumbnailUrl": "/api/v1/thumbnails/root-1/Temporada%201/Episodio%2001.mp4",
      "subtitles": [
        {
          "id": "sub-0",
          "label": "pt-BR",
          "lang": "pt-BR",
          "format": "srt",
          "relPath": "Temporada 1/Episodio 01.pt-BR.srt",
          "srcUrl": "/api/v1/subtitles/root-1/Temporada%201/Episodio%2001.pt-BR.srt"
        }
      ]
    }
  ]
}
```
- **Error Responses**:
  - `400 Bad Request`: Parâmetro `root` inválido.
  - `403 Forbidden`: Caminho contém tentativa de *path traversal* ou link simbólico apontando para fora da raiz (`FORBIDDEN_PATH`).
  - `404 Not Found`: Pasta não encontrada (`NOT_FOUND`).

---

### 2.3 `GET /api/v1/search`

Busca vídeos pelo nome (insensível a maiúsculas/minúsculas) em todas as raízes configuradas ou dentro de uma raiz específica (`FR-011`).

- **Query Parameters**:
  - `q` (obrigatório): Termo de busca (ex.: `episodio`).
  - `root` (opcional): Filtra apenas dentro da raiz informada.
  - `sort` (opcional, padrão `name_asc`): `name_asc` | `name_desc` | `date_desc` | `date_asc`.

- **Response `200 OK`** (`application/json`):
```json
{
  "query": "episodio",
  "total": 1,
  "videos": [
    {
      "id": "root-1:Temporada 1/Episodio 01.mp4",
      "rootId": "root-1",
      "rootLabel": "Séries",
      "name": "Episodio 01.mp4",
      "title": "Episodio 01",
      "relPath": "Temporada 1/Episodio 01.mp4",
      "folderRelPath": "Temporada 1",
      "sizeBytes": 157286400,
      "modifiedAt": "2026-11-19T18:00:00Z",
      "mimeType": "video/mp4",
      "streamUrl": "/api/v1/stream/root-1/Temporada%201/Episodio%2001.mp4",
      "thumbnailUrl": "/api/v1/thumbnails/root-1/Temporada%201/Episodio%2001.mp4",
      "subtitles": []
    }
  ]
}
```

---

### 2.4 `GET /api/v1/stream/{rootId}/{path...}`

Transmite o arquivo de vídeo com suporte completo a **HTTP Range Requests** (`FR-006`, `SC-003`).

- **Request Headers suportados**:
  - `Range: bytes=<start>-<end>` (RFC 9110)
  - `If-Modified-Since`
- **Response Headers**:
  - `Accept-Ranges: bytes`
  - `Content-Type: <mimeType>`
  - `Content-Length: <chunk-or-full-size>`
  - `Content-Range: bytes <start>-<end>/<total>` (quando `206 Partial Content`)
  - `Last-Modified: <RFC1123>`
- **Status Codes**:
  - `200 OK`: Transmissão completa quando `Range` não é enviado.
  - `206 Partial Content`: Transmissão do intervalo solicitado para início rápido e *seek*.
  - `416 Range Not Satisfiable`: Intervalo fora do tamanho do arquivo.
  - `403 Forbidden`: Tentativa de acesso fora da raiz ou arquivo não permitido.
  - `404 Not Found`: Vídeo não encontrado ou removido do disco (`FR-017`).

---

### 2.5 `GET /api/v1/subtitles/{rootId}/{path...}`

Serve legendas locais `.vtt` ou `.srt` sempre no formato `text/vtt; charset=utf-8` (`FR-014`).

- **Behavior**:
  - Se o arquivo for `.vtt`, transmite diretamente com `Content-Type: text/vtt; charset=utf-8`.
  - Se o arquivo for `.srt`, converte em memória para WebVTT (`WEBVTT` + normalização de timestamps `,` → `.`) sem gravar nada no disco (`FR-008`).
- **Status Codes**:
  - `200 OK`: Conteúdo WebVTT válido.
  - `403 Forbidden`: Caminho fora da raiz ou extensão não permitida.
  - `404 Not Found`: Legenda não encontrada.

---

### 2.6 `GET /api/v1/thumbnails/{rootId}/{path...}`

Retorna uma miniatura visual para o vídeo especificado sem escrever nas pastas de mídia (`FR-005`, `FR-008`).

- **Behavior**:
  1. Se existir imagem sidecar somente-leitura ao lado do vídeo (`.jpg`, `.jpeg`, `.png`, `.webp`), retorna `200 OK` com a imagem.
  2. Se `ffmpeg` estiver disponível no servidor e miniaturas não estiverem desabilitadas, extrai 1 frame JPEG para o diretório efêmero `--cache-dir` (fora das pastas de mídia) e retorna `200 OK` (`image/jpeg`).
  3. Caso contrário, retorna `204 No Content` imediatamente para que a Web UI utilize captura local ou o cartão SVG padrão sem erro de console.
- **Status Codes**:
  - `200 OK`: Imagem (`image/jpeg`, `image/png` ou `image/webp`) com `Cache-Control: public, max-age=3600`.
  - `204 No Content`: Miniatura de servidor indisponível (fallback gracioso no cliente).
  - `403 Forbidden` / `404 Not Found`: Caminho inválido ou vídeo inexistente.

---

### 2.7 Rotas Estáticas e Conformidade AGPL v3.0 (`FR-010`, `FR-016`)

- `GET /` e `GET /assets/*`: Servem os arquivos da Web UI embutidos no binário via `embed.FS` com `Content-Security-Policy: default-src 'self'; media-src 'self' blob:; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'` (garantindo 0 requisições externas).
- `GET /LICENSE`: Serve o texto integral da licença `GNU AGPL v3.0` embutido no binário.
