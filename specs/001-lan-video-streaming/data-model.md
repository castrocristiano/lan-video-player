# Data Model: Player de Vídeo em Rede Local via Navegador

**Feature**: `001-lan-video-streaming`
**Date**: 2026-11-19

---

## Visão Geral

O `lan-video-player` não utiliza banco de dados relacional externo (em conformidade com o Princípio III da Constituição). Os dados do servidor são derivados diretamente da leitura em tempo real (somente-leitura) do sistema de arquivos das raízes configuradas, enquanto o estado de sessão e progresso de cada espectador reside localmente no navegador do próprio dispositivo (`localStorage`).

---

## 1. Entidades do Servidor (Go)

### 1.1 `MediaRoot` (Raiz de Biblioteca)

Representa um diretório de mídia autorizado configurado pelo operador na inicialização via flag de linha de comando (`--media`) / variável de ambiente (`LVP_MEDIA_DIRS`) ou adicionado dinamicamente pelo seletor visual de pastas na Web UI (`FR-001`, `FR-018`, `FR-019`).

| Campo | Tipo | Descrição | Regras de Validação |
|-------|------|-----------|---------------------|
| `ID` | `string` | Identificador curto determinístico (ex.: `root-1`, `root-2` ou hash curto do caminho canônico) usado nas rotas da API. | Único entre todas as raízes ativas; apenas `[a-z0-9-]`. |
| `Label` | `string` | Nome amigável exibido na interface (por padrão o `filepath.Base` do diretório; se houver colisão de nomes entre duas raízes, recebe sufixo diferenciador ex.: `Filmes (1)`, `Filmes (2)`). | Não vazio (`Edge Case: Pasta configurada inválida ou duplicada`). |
| `CanonicalPath` | `string` | Caminho absoluto resolvido via `filepath.Abs` + `filepath.EvalSymlinks`. Nunca exposto nas respostas públicas de listagem de vídeos. | Deve existir, ser um diretório (`IsDir() == true`) e possuir permissão de leitura (`FR-002`, `FR-018`). |
| `RelToBrowseBase` | `string` | Caminho relativo ao `--browse-base` (quando a raiz estiver dentro do diretório-base do seletor visual). | Usado pela Web UI para indicar quais pastas do seletor já estão adicionadas à biblioteca. |
| `Source` | `string` | Origem do registro da raiz: `"cli"` (fixada por `--media` / `LVP_MEDIA_DIRS`) ou `"ui"` (adicionada pelo usuário via Web UI e salva em `library.json`). | `"cli"` ou `"ui"`. |
| `Removable` | `bool` | Indica se a raiz pode ser desvinculada pela interface web (`true` para `"ui"`, `false` para `"cli"`). | Desvincular remove apenas do registro da biblioteca, nunca exclui arquivos do disco (`FR-018`). |

### 1.2 `LibraryConfig` (Configuração Persistente da Biblioteca)

Arquivo JSON persistido fora das pastas de mídia (`~/.config/lan-video-player/library.json` ou `--config-file` / `LVP_CONFIG_FILE`) que armazena as pastas adicionadas pelo seletor visual da Web UI (`FR-019`).

| Campo | Tipo | Descrição | Regras de Validação |
|-------|------|-----------|---------------------|
| `Version` | `int` | Versão do schema do arquivo de configuração (`1`). | Sempre `1` na v1. |
| `Roots` | `[]SavedRootEntry` | Lista de pastas adicionadas pela Web UI (`{ "path": "...", "label": "...", "addedAt": "..." }`). | Cada `path` deve estar contido dentro do `--browse-base` autorizado e ser validado por `safefs` ao carregar. |

### 1.3 `PickerDirEntry` (Item do Seletor Visual de Pastas)

Representa um diretório listado no modal "Adicionar Pastas" dentro do `--browse-base` (`FR-018`).

| Campo | Tipo | Descrição | Regras de Validação |
|-------|------|-----------|---------------------|
| `Name` | `string` | Nome da pasta. | Pastas ocultas (iniciadas por `.`) são omitidas. |
| `RelPath` | `string` | Caminho relativo ao `--browse-base` (ex.: `Videos/Filmes`). | Validado contra *path traversal* e symlinks fora do `--browse-base`. |
| `HasSubdirs` | `bool` | Indica se possui subpastas navegáveis no seletor. | Somente leitura. |
| `AlreadyAdded` | `bool` | Indica se esta pasta já faz parte da biblioteca ativa. | Calculado comparando `CanonicalPath` com as `MediaRoot` ativas. |

### 1.4 `BreadcrumbItem` (Item da Trilha de Navegação)

Representa um nível hierárquico para navegação rápida (`FR-004`).

| Campo | Tipo | Descrição | Regras de Validação |
|-------|------|-----------|---------------------|
| `Label` | `string` | Nome de exibição do nível (nome da raiz ou da subpasta). | Não vazio. |
| `RootID` | `string` | Identificador da raiz à qual o nível pertence. | Deve corresponder a um `MediaRoot.ID` válido. |
| `RelPath` | `string` | Caminho relativo normalizado com `/` a partir da raiz (`""` para a própria raiz). | Sem `..`, sem barra inicial. |

### 1.5 `FolderEntry` (Pasta / Subpasta)

Representa um subdiretório dentro de uma `MediaRoot` (`FR-003`, `FR-004`).

| Campo | Tipo | Descrição | Regras de Validação |
|-------|------|-----------|---------------------|
| `Name` | `string` | Nome do diretório. | Não pode começar com `.` (arquivos/pastas ocultos são ignorados por padrão, `FR-003`). |
| `RootID` | `string` | ID da `MediaRoot` a que pertence. | Deve existir. |
| `RelPath` | `string` | Caminho relativo seguro dentro da raiz (separador `/`). | Validado contra *path traversal* e symlinks externos (`FR-009`). |
| `ModifiedAt` | `time.Time` (ISO-8601) | Data/hora da última modificação do diretório. | Somente leitura via `os.Stat`. |

### 1.6 `VideoItem` (Item de Vídeo)

Representa um arquivo de vídeo reconhecido dentro de uma `MediaRoot` (`FR-003`, `FR-005`, `FR-006`).

| Campo | Tipo | Descrição | Regras de Validação |
|-------|------|-----------|---------------------|
| `ID` | `string` | Chave composta canônica (`{rootId}:{relPath}`) usada pelo cliente para rastrear progresso e seleção. | Determinística e estável. |
| `RootID` | `string` | ID da `MediaRoot` onde o vídeo está localizado. | Deve corresponder a uma raiz registrada. |
| `RootLabel` | `string` | Rótulo da raiz (útil nos resultados de busca global, `FR-011`). | Não vazio. |
| `Name` | `string` | Nome do arquivo com extensão (ex.: `Episodio 01.mp4`). | Não pode ser arquivo oculto (iniciado por `.`). |
| `Title` | `string` | Título legível (nome do arquivo sem extensão). | UTF-8 válido (`Edge Case: Nomes com caracteres especiais`). |
| `RelPath` | `string` | Caminho relativo à raiz (ex.: `Temporada 1/Episodio 01.mp4`). | Canonicalizado e contido estritamente em `MediaRoot.CanonicalPath`. |
| `FolderRelPath` | `string` | Caminho relativo da pasta pai (ex.: `Temporada 1`). | `""` quando na raiz. |
| `SizeBytes` | `int64` | Tamanho do arquivo em bytes. | `>= 0`. |
| `ModifiedAt` | `time.Time` (ISO-8601) | Data/hora de modificação do arquivo. | Obtido via `os.Stat`. |
| `MimeType` | `string` | Tipo MIME mapeado pela extensão (`.mp4` → `video/mp4`, `.webm` → `video/webm`, `.mkv` → `video/x-matroska`, `.mov` → `video/quicktime`, `.m4v` → `video/mp4`, `.ogv` → `video/ogg`, `.avi` → `video/x-msvideo`). | Apenas extensões de vídeo suportadas são listadas (`FR-003`). |
| `StreamURL` | `string` | URL relativa local para streaming HTTP Range (`/api/v1/stream/{rootId}/{relPath}`). | Gerada localmente com encoding de segmento URL. |
| `ThumbnailURL` | `string` | URL relativa local para miniatura (`/api/v1/thumbnails/{rootId}/{relPath}`). | Nunca aponta para URL externa (`FR-010`). |
| `Subtitles` | `[]SubtitleTrack` | Lista de legendas externas locais associadas ao vídeo (`FR-014`). | Ordenada por rótulo/idioma. |

### 1.7 `SubtitleTrack` (Legenda Local)

Representa um arquivo de legenda `.vtt` ou `.srt` na mesma pasta e com o mesmo nome base do vídeo (`FR-014`).

| Campo | Tipo | Descrição | Regras de Validação |
|-------|------|-----------|---------------------|
| `ID` | `string` | Identificador da faixa (ex.: `sub-0`, `sub-1`). | Único por vídeo. |
| `Label` | `string` | Rótulo amigável extraído do sufixo do arquivo (ex.: `filme.pt-BR.srt` → `pt-BR`; `filme.srt` → `Padrão`). | Não vazio. |
| `Lang` | `string` | Código de idioma inferido do sufixo (ex.: `pt-BR`, `en`, `es` ou `und` quando ausente). | Formato BCP-47 simples ou `und`. |
| `Format` | `string` | Formato original no disco (`vtt` ou `srt`). | Apenas `.vtt` e `.srt` reconhecidos. |
| `RelPath` | `string` | Caminho relativo do arquivo de legenda dentro da raiz. | Validado pela mesma camada `safefs` (`FR-009`). |
| `SrcURL` | `string` | URL local que entrega a legenda sempre em formato `text/vtt` (`/api/v1/subtitles/{rootId}/{relPath}`). | Conversão em memória para `.srt` sem escrita em disco (`FR-008`). |

---

## 2. Entidades do Cliente (Navegador / `localStorage`)

### 2.1 `PlaybackProgress` (Progresso de Reprodução)

Armazenado localmente no navegador do dispositivo em `localStorage` sob a chave `lvp:progress:{rootId}:{relPath}` (`FR-012`, `SC-007`).

| Campo | Tipo | Descrição | Regras de Transição de Estado |
|-------|------|-----------|-------------------------------|
| `videoId` | `string` | `{rootId}:{relPath}` | Chave primária lógica no dispositivo. |
| `currentTime` | `number` | Último instante assistido em segundos (float). | Atualizado a cada ~2–5 segundos durante reprodução ativa e no evento `pause`/`pagehide`. |
| `duration` | `number` | Duração total conhecida do vídeo em segundos. | `> 0`. |
| `progressRatio` | `number` | Razão `currentTime / duration` (`0.0` a `1.0`). | Usado para renderizar a barra de progresso no cartão do vídeo. |
| `completed` | `boolean` | Indica se o vídeo foi concluído. | Quando `currentTime / duration >= 0.95` ou evento `ended` dispara, `completed = true` e `currentTime = 0` para que a próxima abertura inicie do começo (User Story 4 Cenário 2). |
| `updatedAt` | `number` | Timestamp Unix em milissegundos da última atualização. | Usado para ordenação ou limpeza opcional de cache local. |

### 2.2 `PlayerPreferences` (Preferências do Espectador no Dispositivo)

Armazenado em `localStorage` sob a chave `lvp:prefs` (`FR-007`, `FR-011`, `FR-013`).

| Campo | Tipo | Padrão | Descrição |
|-------|------|--------|-----------|
| `playbackRate` | `number` | `1.0` | Velocidade de reprodução (`0.5`, `0.75`, `1.0`, `1.25`, `1.5`, `2.0`). |
| `autoPlayNext` | `boolean` | `true` | Ativa/desativa contagem regressiva para próximo vídeo automático ao final da reprodução (`FR-013`). |
| `sortBy` | `string` | `"name_asc"` | Critério de ordenação atual (`name_asc`, `name_desc`, `date_desc`, `date_asc`) (`FR-011`). |
| `volume` | `number` | `1.0` | Volume preferido (`0.0` a `1.0`). |
| `muted` | `boolean` | `false` | Estado de silenciamento. |

---

## 3. Relacionamentos e Ciclo de Vida

```text
[MediaRoot 1..N] (somente-leitura no servidor)
   ├── contém 0..N [FolderEntry] (hierarquia recursiva validada por safefs)
   └── contém 0..N [VideoItem]
          ├── associado a 0..N [SubtitleTrack] (mesma pasta + mesmo nome base)
          └── referenciado no navegador por 0..1 [PlaybackProgress] (por dispositivo em localStorage)
```
