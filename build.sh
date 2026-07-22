#!/usr/bin/env bash
# Builds the admin frontend and embeds it into a single self-contained
# server binary (API + web UI, one executable).
#
# Usage:    ./build.sh
# Requires: Node.js/npm (for the frontend) and Go 1.23+ (for the server).
# Run from anywhere -- paths below are resolved relative to this script.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> Building admin frontend..."
cd "$root/admin"
[ -d node_modules ] || npm install

# Clear previous build output but keep the two files that let a fresh
# checkout compile the Go module before any frontend build has ever run
# (see server/internal/webui/webui.go) -- Vite's own emptyOutDir would
# delete these too (see vite.config.ts), so this script owns the cleanup
# instead.
dist_dir="$root/server/internal/webui/dist"
if [ -d "$dist_dir" ]; then
  find "$dist_dir" -mindepth 1 ! -name ".gitkeep" ! -name ".gitignore" -exec rm -rf {} +
fi

npm run build

echo "==> Building server binary (with embedded admin UI)..."
cd "$root/server"
# The sqlite driver (github.com/glebarez/sqlite, backed by
# modernc.org/sqlite) is pure Go, not cgo -- no C compiler needed, and this
# also produces a fully static binary.
export CGO_ENABLED=0
go build -o ../ubibot-server ./cmd/server

echo ""
echo "==> Done: ubibot-server"
echo "    Run it from the repo root, e.g.: ./ubibot-server"
echo "    Admin UI + API are both served from the same address (default :8080)."
