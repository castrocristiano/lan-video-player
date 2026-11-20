# 02 — Streaming, Transcodificação FFmpeg e Player

## Streaming Direto com HTTP Range (`206 Partial Content`)

Para formatos nativos de navegador (`.mp4`, `.webm`, `.m4v`, `.mov`, `.ogv`), o endpoint `/api/v1/stream/{rootId}/{path...}` utiliza `http.ServeContent` sobre o descritor somente-leitura, entregando:

- `Accept-Ranges: bytes` e respostas `206 Partial Content`.
- Início instantâneo da reprodução sem baixar o arquivo completo.
- Salto imediato (*seek*) para qualquer ponto da linha do tempo.

---

## Transcodificação em Tempo Real para Formatos Legados

Para contêineres antigos que os navegadores não decodificam nativamente (`.avi`, `.rmvb`, `.rm`, `.wmv`, `.flv`, `.mpg`, `.mpeg`, `.ts`, `.m2ts`) ou quando ocorre fallback de codec (`?transcode=1`), o servidor utiliza `ffmpeg` (quando disponível no host ou no container Alpine) para transmitir em tempo real um stream **MP4 fragmentado (`H.264 + AAC`)** via `pipe:1` em memória:

- **Zero escrita em disco**: Nenhuma conversão prévia ou arquivo temporário é gravado nas pastas de mídia.
- **Seek em streams transcodificados**: Antes de abrir o vídeo, o player consulta `GET /api/v1/probe/{rootId}/{path...}` (`ffprobe`) para obter a duração total exata do arquivo. Ao arrastar a barra de progresso para qualquer minuto (ex.: minuto `10:00`), o player solicita `?transcode=1&start=600` e o `ffmpeg -ss` reposiciona diretamente no instante escolhido.

---

## Player Web (Desktop, Mobile e Smart TV)

- **Tela Cheia estilo YouTube**: O vídeo ocupa `100vw × 100vh`; o cabeçalho e a barra de controles aparecem com gradiente translúcido ao mover o mouse e ocultam-se automaticamente após 2,5s de inatividade durante a reprodução.
- **Controles completos**: Play/Pause, `-10s` / `+10s`, barra de progresso com tempo atual/total, volume/mudo, velocidade (`0.5x` a `2.0x`), legendas (`.vtt` e conversão em memória de `.srt`), seletor de faixa de áudio e próximo vídeo automático com contagem regressiva cancelável.
- **Atalhos**: `Espaço`/`K` (Play/Pause), `←`/`→` (`-10s`/`+10s`), `F` (Tela Cheia), `Esc` (Fechar).
- **Retomada por Dispositivo**: Progresso salvo em `localStorage` (`lvp:progress:{rootId}:{relPath}`), com reset automático ao atingir `>= 95%`.
