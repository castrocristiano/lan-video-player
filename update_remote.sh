#!/usr/bin/env bash
# ==============================================================================
# Script: update_remote.sh
# Finalidade: Conectar via SSH ao servidor dc1 (192.168.1.20), verificar se há
# nova release no GitHub (ou sincronizar a branch atual quando ainda não houver
# release publicada), preservar o arquivo .env e reconstruir/reiniciar o
# container via Podman Compose / Docker Compose na porta 8090.
# ==============================================================================

set -euo pipefail

REMOTE_HOST="${REMOTE_HOST:-192.168.1.20}"
REMOTE_USER="${REMOTE_USER:-cristiano}"
REMOTE_DIR="${REMOTE_DIR:-/home/cristiano/dev/player}"
GITHUB_REPO="${GITHUB_REPO:-castrocristiano/lan-video-player}"
FORCE="${FORCE:-false}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_ed25519}"

SSH_OPTS=(-o ConnectTimeout=10)
if [ -f "$SSH_KEY" ]; then
    SSH_OPTS+=(-i "$SSH_KEY")
fi

echo "=========================================================="
echo "🚀 Verificando atualizações no servidor $REMOTE_USER@$REMOTE_HOST"
echo "   Diretório remoto: $REMOTE_DIR"
echo "=========================================================="

# Se o diretório local for um repositório git, sincroniza os commits locais
# diretamente via SSH para garantir que o dc1 tenha o código mais recente
# mesmo antes da primeira Release pública no GitHub.
if git rev-parse --git-dir >/dev/null 2>&1; then
    CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
    echo "🔄 Sincronizando branch local '$CURRENT_BRANCH' para $REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR..."
    git -c core.sshCommand="ssh ${SSH_OPTS[*]}" push -f "ssh://$REMOTE_USER@$REMOTE_HOST$REMOTE_DIR" "$CURRENT_BRANCH:refs/heads/$CURRENT_BRANCH" 2>/dev/null || true
fi

ssh "${SSH_OPTS[@]}" "$REMOTE_USER@$REMOTE_HOST" bash -s -- "$GITHUB_REPO" "$REMOTE_DIR" "$FORCE" << 'REMOTE_EOF'
set -euo pipefail

REPO="$1"
TARGET_DIR="$2"
FORCE="$3"

mkdir -p "$TARGET_DIR"
cd "$TARGET_DIR"

echo "🔍 Consultando última release em: https://api.github.com/repos/$REPO/releases/latest"
RELEASE_JSON=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" || echo "")

LATEST_TAG=$(echo "$RELEASE_JSON" | grep -Po '"tag_name":\s*"\K[^"]+' || echo "")
TARBALL_URL=$(echo "$RELEASE_JSON" | grep -Po '"tarball_url":\s*"\K[^"]+' || echo "")

VERSION_FILE="$TARGET_DIR/.current_release_version"
CURRENT_VERSION=""
if [ -f "$VERSION_FILE" ]; then
    CURRENT_VERSION=$(cat "$VERSION_FILE")
fi

if [ -n "$LATEST_TAG" ] && [ -n "$TARBALL_URL" ]; then
    echo "📦 Release mais recente no GitHub: $LATEST_TAG (Instalada: ${CURRENT_VERSION:-Nenhuma})"

    if [ "$LATEST_TAG" == "$CURRENT_VERSION" ] && [ "$FORCE" != "true" ] && [ "$FORCE" != "1" ]; then
        echo "✅ O servidor já está rodando a release mais recente ($LATEST_TAG)."
        echo "💡 Para forçar o rebuild: FORCE=1 ./update_remote.sh"
        exit 0
    fi

    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    ARCHIVE_FILE="$TMP_DIR/release.tar.gz"
    curl -sL "$TARBALL_URL" -o "$ARCHIVE_FILE"

    if [ -f "$TARGET_DIR/.env" ]; then
        cp "$TARGET_DIR/.env" "$TMP_DIR/.env.backup"
    fi

    EXTRACT_DIR="$TMP_DIR/extracted"
    mkdir -p "$EXTRACT_DIR"
    tar -xzf "$ARCHIVE_FILE" -C "$EXTRACT_DIR" --strip-components=1

    rm -rf "$TARGET_DIR/cmd" "$TARGET_DIR/internal" "$TARGET_DIR/web"
    cp -rf "$EXTRACT_DIR"/* "$TARGET_DIR"/
    cp -rf "$EXTRACT_DIR"/.[!.]* "$TARGET_DIR"/ 2>/dev/null || true

    if [ -f "$TMP_DIR/.env.backup" ]; then
        cp "$TMP_DIR/.env.backup" "$TARGET_DIR/.env"
        echo "🔒 Arquivo .env preservado com sucesso."
    fi

    echo "$LATEST_TAG" > "$VERSION_FILE"
else
    echo "ℹ️  Nenhuma release publicada na API do GitHub (ou repositório privado). Utilizando código sincronizado via Git local..."
    if [ -d "$TARGET_DIR/.git" ]; then
        if git show-ref --verify --quiet refs/heads/001-lan-video-streaming; then
            git checkout -f 001-lan-video-streaming
        elif git show-ref --verify --quiet refs/heads/main; then
            git checkout -f main
        fi
        echo "$(git rev-parse --short HEAD)" > "$VERSION_FILE"
    fi
fi

if [ ! -f "$TARGET_DIR/.env" ] && [ -f "$TARGET_DIR/.env.example" ]; then
    cp "$TARGET_DIR/.env.example" "$TARGET_DIR/.env"
    echo "📝 Arquivo .env criado a partir de .env.example."
fi

echo "🔄 Reconstruindo e reiniciando container lan-video-player..."
if command -v podman-compose &> /dev/null; then
    podman-compose down || true
    podman-compose up -d --build
elif command -v podman &> /dev/null && podman compose version &> /dev/null; then
    podman compose down || true
    podman compose up -d --build
elif command -v docker-compose &> /dev/null; then
    docker-compose down || true
    docker-compose up -d --build
else
    echo "⚠️ Nem podman-compose nem docker-compose foram encontrados no servidor remoto."
    exit 1
fi

echo "=========================================================="
echo "🎉 Deploy concluído com sucesso no dc1!"
echo "   LAN:       http://192.168.1.20:8090"
echo "   Tailscale: http://dc1.tailc8fc9a.ts.net:8090"
echo "=========================================================="
REMOTE_EOF
