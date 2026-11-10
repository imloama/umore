#!/usr/bin/env bash
# 一键构建：前端产物拷进 backend/web，后端编成不依赖 cgo 的静态二进制。
# 用法: ./build.sh   产物: backend/umore、backend/umore-migrate、backend/web/
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
FRONTEND="$ROOT/frontend"
BACKEND="$ROOT/backend"

echo "==> build frontend"
cd "$FRONTEND"
if [ -f package-lock.json ]; then
  npm ci
else
  npm install
fi
npm run build

echo "==> copy dist -> backend/web"
rm -rf "$BACKEND/web"
mkdir -p "$BACKEND/web"
cp -R "$FRONTEND/dist/." "$BACKEND/web/"

echo "==> build backend"
cd "$BACKEND"
CGO_ENABLED=0 go build -trimpath -o umore ./cmd/server
CGO_ENABLED=0 go build -trimpath -o umore-migrate ./cmd/migrate

echo "==> done"
ls -lh "$BACKEND/umore" "$BACKEND/umore-migrate"
echo "启动: ./start.sh   或   cd backend && ./umore"
