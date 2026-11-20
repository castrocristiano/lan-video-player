# Quickstart & Validation Guide: `lan-video-player`

**Feature**: `001-lan-video-streaming`
**Date**: 2026-11-19

---

## Pré-requisitos

- **Opção A (Execução Direta do Binário)**:
  - Go `1.23+` instalado para compilar (o binário resultante não exige Go instalado para rodar).
  - Opcional: `ffmpeg` instalado no `PATH` do sistema caso deseje extração de miniaturas no servidor (sem `ffmpeg`, o player funciona normalmente com fallback visual na Web UI).
- **Opção B (Execução em Container via Podman / Docker)**:
  - `podman` (`4.x+` ou `5.x+`) com `podman compose` / `podman-compose` (ou `docker compose`).
- Uma ou mais pastas locais contendo arquivos de vídeo (`.mp4`, `.webm`, `.mkv`, etc.) e opcionalmente legendas (`.srt` ou `.vtt`).

---

## 1. Compilação e Execução Direta (Binário Único)

### 1.1 Compilar o binário estático

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/lan-video-player ./cmd/lan-video-player
```

### 1.2 Iniciar com uma ou mais pastas de mídia

```bash
./bin/lan-video-player \
  --addr 0.0.0.0:8080 \
  --media /caminho/para/Filmes \
  --media /caminho/para/Series
```

Ou via variáveis de ambiente:

```bash
LVP_ADDR="0.0.0.0:8080" \
LVP_MEDIA_DIRS="/caminho/para/Filmes:/caminho/para/Series" \
./bin/lan-video-player
```

Acesse em qualquer navegador na mesma rede local:
- No próprio computador: `http://localhost:8080`
- Em celulares, tablets ou Smart TVs na LAN: `http://<IP-DO-COMPUTADOR>:8080`

---

## 2. Execução via Container com `podman compose` (Rootless)

O arquivo `compose.yaml` na raiz do repositório é projetado para rodar em **Podman rootless** com usuário não-root, filesystem raiz do container somente-leitura e montagem dos diretórios de mídia com `:ro,Z` (compatível com SELinux no Fedora/RHEL/CentOS e Ubuntu/Debian).

### 2.1 Subir o serviço com Podman Compose

```bash
# Defina as pastas de mídia no ambiente ou arquivo .env
export LVP_HOST_MEDIA_1=/caminho/para/Filmes
export LVP_HOST_MEDIA_2=/caminho/para/Series
export LVP_PORT=8080

podman compose up -d --build
```

*(Compatível também com `podman-compose up -d --build` ou `docker compose up -d --build`.)*

### 2.2 Verificar logs e status

```bash
podman compose ps
podman compose logs -f
```

---

## 3. Execução da Suíte de Testes Automatizados

Para validar todos os portões de qualidade exigidos pela Constituição (isolamento contra *path traversal*, symlinks externos, garantia somente-leitura, listagem, busca, legendas `.srt` → `.vtt` e streaming HTTP Range `206 Partial Content`):

```bash
go test -v -race ./...
```

---

## 4. Cenários de Validação Ponta a Ponta (Smoke Checks)

1. **Validação de Raízes, Seletor Visual de Pastas e Navegação (User Story 1 / `FR-018`, `FR-019`, `SC-001`, `SC-002`)**:
   - Abra `http://localhost:8080`, clique em **"Gerenciar Biblioteca / Adicionar Pastas"**, navegue pelo diretório-base seguro, marque uma ou mais pastas de vídeos e clique em **Adicionar**.
   - Reinicie o serviço e confirme que as pastas adicionadas permanecem na biblioteca automaticamente (carregadas de `library.json`).
   - Confirme que as subpastas navegam via breadcrumbs em `< 1s` e arquivos não-vídeo/ocultos não são exibidos.
   - Consulte `curl -s http://localhost:8080/api/v1/roots` para verificar o contrato JSON documentado em `contracts/http-api.md`.
2. **Validação de Segurança e Somente-Leitura (`SC-005`)**:
   - Execute `curl -i "http://localhost:8080/api/v1/browse?root=root-1&path=../../etc"` e confirme retorno `403 Forbidden` (`FORBIDDEN_PATH`).
   - Verifique com `find /caminho/para/Filmes -mmin -5` após navegar, gerar miniaturas e assistir vídeos que **zero** arquivos foram criados ou alterados nas pastas de mídia.
3. **Validação de Streaming e Seek via HTTP Range (User Story 2 / `SC-003`)**:
   - Reproduza um vídeo no navegador, salte para 70% da barra de progresso e altere a velocidade para `1.5x`.
   - Teste via CLI: `curl -I -H "Range: bytes=0-1023" "http://localhost:8080/api/v1/stream/root-1/<video>.mp4"` e confirme status `HTTP/1.1 206 Partial Content`, `Accept-Ranges: bytes` e `Content-Range`.
4. **Validação de Busca, Ordenação e Miniaturas (User Story 3)**:
   - Digite parte do nome de um vídeo na barra de busca, alterne a ordenação entre Nome (`A–Z` / `Z–A`) e Data (`Mais recentes`) e confirme a exibição das miniaturas (ou fallback SVG limpo).
5. **Validação de Retomada, Próximo Vídeo e Legendas (User Story 4 / `SC-007`)**:
   - Assista 30 segundos de um vídeo, recarregue a página e confirme que a barra de progresso aparece no cartão do vídeo e a reprodução retoma do ponto salvo (`± 5s`).
   - Coloque um arquivo `<video>.pt-BR.srt` ao lado de `<video>.mp4`, abra o vídeo e ative a legenda `pt-BR` nos controles do player.
6. **Validação 100% Offline (`SC-006`)**:
   - Abra a aba *Network* do DevTools do navegador (com filtro para domínios externos ou simulando LAN sem gateway de internet) e confirme que 100% das requisições são servidas pelo próprio host local (`Content-Security-Policy: default-src 'self'`).
