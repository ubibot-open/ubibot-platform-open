# Builds the admin frontend and embeds it into a single self-contained
# server binary (API + web UI, one .exe).
#
# Usage:   powershell -ExecutionPolicy Bypass -File build.ps1
# Requires: Node.js/npm (for the frontend) and Go 1.23+ (for the server).
# Run from anywhere -- paths below are resolved relative to this script.

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot

Write-Host "==> Building admin frontend..."
Push-Location (Join-Path $root "admin")
try {
    if (-not (Test-Path "node_modules")) {
        npm install
    }

    # Clear previous build output but keep the two files that let a fresh
    # checkout compile the Go module before any frontend build has ever
    # run (see server/internal/webui/webui.go) -- Vite's own emptyOutDir
    # would delete these too (see vite.config.ts), so this script owns the
    # cleanup instead.
    $distDir = Join-Path $root "server/internal/webui/dist"
    if (Test-Path $distDir) {
        Get-ChildItem -Path $distDir -Force |
            Where-Object { $_.Name -ne ".gitkeep" -and $_.Name -ne ".gitignore" } |
            Remove-Item -Recurse -Force
    }

    npm run build
}
finally {
    Pop-Location
}

Write-Host "==> Building server binary (with embedded admin UI)..."
Push-Location (Join-Path $root "server")
try {
    # Force the build target explicitly instead of trusting whatever
    # GOOS/GOARCH happen to already be set in this shell -- a stray
    # GOOS=linux (or similar) left over from unrelated cross-compiling
    # work is exactly what produces a .exe that Windows refuses to run
    # with "This app can't run on your PC" (it silently builds a
    # non-Windows binary and just names it .exe).
    $env:GOOS = "windows"
    switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
        "Arm64" { $env:GOARCH = "arm64" }
        "X86" { $env:GOARCH = "386" }
        default { $env:GOARCH = "amd64" }
    }

    # The sqlite driver (github.com/glebarez/sqlite, backed by
    # modernc.org/sqlite) is pure Go, not cgo -- so no C compiler is
    # needed on the build machine at all, and CGO_ENABLED=0 is not just
    # safe but preferred here: it also produces a fully static binary
    # with no runtime dependency on a mingw/MSVC DLL. (An earlier version
    # of this script required a matching mingw-w64 gcc for a cgo-based
    # sqlite driver -- a mismatched Go/gcc combo on Windows produces a
    # build that "succeeds" but whose .exe Windows refuses to run at all,
    # which is exactly the kind of hard-to-diagnose failure switching to
    # a pure-Go driver avoids for good.)
    $env:CGO_ENABLED = "0"

    Write-Host "    target: GOOS=$($env:GOOS) GOARCH=$($env:GOARCH) CGO_ENABLED=$($env:CGO_ENABLED)"
    go build -o "../ubibot-server.exe" ./cmd/server
}
finally {
    Pop-Location
}

Write-Host ""
Write-Host "==> Done: ubibot-server.exe"
Write-Host "    Run it from the repo root, e.g.: .\ubibot-server.exe"
Write-Host "    Admin UI + API are both served from the same address (default :8080)."
