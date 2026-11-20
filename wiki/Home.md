# LAN Video Player — Wiki

Bem-vindo à documentação técnica e operacional do **lan-video-player**, um servidor e player de vídeo **100% offline para rede local (LAN)**, distribuído como **binário único em Go** com **Web UI embutida (`//go:embed`)** e acesso **estritamente somente-leitura** às suas pastas de mídia.

---

## Princípios Fundamentais (Constituição)

1. **Local-First e Zero Cloud**: Nenhuma telemetria, CDN externo ou dependência de internet (`Content-Security-Policy: default-src 'self'`).
2. **Segurança e Acesso Somente-Leitura**: Nenhum arquivo de mídia do usuário é criado, alterado, renomeado ou excluído; caminhos são validados com `filepath.Abs` + `filepath.EvalSymlinks` (`internal/safefs`).
3. **Binário Único e Web UI Leve**: Executável estático em Go (`CGO_ENABLED=0`) com HTML5/CSS/JS embutidos, sem bancos de dados externos.
4. **Simplicidade e YAGNI**: Configuração mínima via flags CLI, variáveis de ambiente ou seletor visual na Web UI.
5. **Conformidade Aberta**: Licenciado sob **GNU AGPL v3.0** (`LICENSE` servido em `/LICENSE`).

---

## Índice da Documentação

- [[01 - Arquitetura e Segurança Somente-Leitura|01-Arquitetura-e-Seguranca]]
- [[02 - Streaming, Transcodificação FFmpeg e Player|02-Streaming-Transcodificacao-e-Player]]
- [[03 - Seletor Visual de Pastas e API HTTP|03-Seletor-de-Pastas-e-API-HTTP]]
- [[04 - Guia de Instalação, Docker e Podman Rootless|04-Guia-de-Instalacao-e-Podman]]
- [[05 - Metodologia Spec-Driven (SDD) e CI/CD|05-Metodologia-Spec-Driven-e-CI-CD]]
