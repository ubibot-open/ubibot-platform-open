import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
export default defineConfig({
    plugins: [react()],
    server: {
        port: 5173,
    },
    build: {
        // Emits straight into the Go module's internal/webui/dist -- that's
        // the go:embed source in server/internal/webui/webui.go, so `npm run
        // build` here followed by `go build` in server/ produces a single
        // binary with the admin UI bundled in.
        //
        // emptyOutDir is deliberately false: that directory has two files
        // (.gitkeep, .gitignore) committed to the repo so a fresh checkout can
        // compile the Go module before the frontend has ever been built (see
        // webui.go) -- Vite's own emptyOutDir would delete them along with
        // everything else. The repo's build script (build.ps1/build.sh)
        // clears out old build output itself, preserving those two files,
        // before running this build.
        outDir: '../server/internal/webui/dist',
        emptyOutDir: false,
    },
});
