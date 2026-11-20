# 04 — Guia de Instalação, Docker e Podman Rootless

## 1. Execução Direta (Binário Único)

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/lan-video-player ./cmd/lan-video-player

./bin/lan-video-player \
  --addr 0.0.0.0:8080 \
  --browse-base "$HOME"
```

---

## 2. Execução em Container (`podman compose` / `docker compose`)

O [compose.yaml](../compose.yaml) já vem configurado para rodar em **Podman rootless** (ou Docker Compose) com:
- Filesystem raiz imutável (`read_only: true`).
- Mapeamento `userns_mode: "keep-id:uid=10001,gid=10001"` para leitura segura de diretórios do host.
- Montagem somente-leitura (`:ro`) de `/home` → `/media/home` e `/mnt` → `/media/mnt`.
- Persistência de configuração em `./config:/config` (`library.json`).
- Porta padrão **`8090`** no host mapeada para `8080` no container.

### Passo a passo

```bash
cp .env.example .env
mkdir -p ./config

# Subir com Podman Compose:
podman compose up -d --build
# ou: podman-compose up -d --build
# ou: docker compose up -d --build
```

Acesse no navegador: `http://<IP-DO-SERVIDOR>:8090`

---

## 3. Variáveis de Ambiente

| Variável | Padrão | Descrição |
|----------|--------|-----------|
| `LVP_PORT` | `8090` | Porta exposta no host pelo `compose.yaml`. |
| `LVP_HOST_HOME` | `/home` | Diretório do host montado em `/media/home:ro`. |
| `LVP_HOST_MNT` | `/mnt` | Diretório de discos montados em `/media/mnt:ro`. |
| `LVP_ADDR` | `0.0.0.0:8080` | Endereço/porta interna do processo Go. |
| `LVP_BROWSE_BASE` | `/media` | Diretório-base do seletor visual de pastas. |
| `LVP_CONFIG_FILE` | `/config/library.json` | Caminho do arquivo de persistência da biblioteca. |
| `LVP_MEDIA_DIRS` | *(vazio)* | Pastas fixas iniciais separadas por `:`. |
| `LVP_NO_THUMBS` | `false` | Desativa extração de miniaturas no servidor. |
