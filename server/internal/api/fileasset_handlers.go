package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

const maxFileAssetUploadBytes = 32 << 20 // 32MB

type fileAssetDTO struct {
	ID        uint   `json:"id"`
	Category  string `json:"category"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	CreatedAt int64  `json:"created_at"`
}

func toFileAssetDTO(f *model.FileAsset) fileAssetDTO {
	return fileAssetDTO{ID: f.ID, Category: f.Category, Filename: f.Filename, Size: f.Size, SHA256: f.SHA256, CreatedAt: f.CreatedAt.Unix()}
}

// UploadFileAsset handles POST /api/admin/files (multipart/form-data:
// category, file) — a generic file registry (exports, attachments) kept
// separate from Firmware, which has its own OTA-specific columns.
func (s *Server) UploadFileAsset(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFileAssetUploadBytes)
	if err := r.ParseMultipartForm(maxFileAssetUploadBytes); err != nil {
		adminErr(w, 400, "invalid upload (file too large or malformed)")
		return
	}
	category := r.FormValue("category")
	if category == "" {
		category = "other"
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		adminErr(w, 400, "file is required")
		return
	}
	defer file.Close()

	if err := os.MkdirAll(s.FileDir, 0o755); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	destName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), header.Filename)
	destPath := filepath.Join(s.FileDir, destName)

	dest, err := os.Create(destPath)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(dest, hasher), file)
	dest.Close()
	if err != nil {
		_ = os.Remove(destPath)
		adminErr(w, 500, "failed to store file")
		return
	}

	f, err := s.Store.CreateFileAsset(category, header.Filename, destPath, size, hex.EncodeToString(hasher.Sum(nil)))
	if err != nil {
		_ = os.Remove(destPath)
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "file.upload", "file_asset", f.ID, header.Filename)
	writeJSON(w, 200, toFileAssetDTO(f))
}

// ListFileAssets handles GET /api/admin/files.
func (s *Server) ListFileAssets(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListFileAssets()
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]fileAssetDTO, 0, len(rows))
	for i := range rows {
		list = append(list, toFileAssetDTO(&rows[i]))
	}
	writeJSON(w, 200, map[string]any{"list": list})
}

// DeleteFileAsset handles DELETE /api/admin/files/{id}.
func (s *Server) DeleteFileAsset(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	if err := s.Store.DeleteFileAsset(uint(id)); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			adminErr(w, 404, "file not found")
			return
		}
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "file.delete", "file_asset", uint(id), "")
	writeJSON(w, 200, map[string]any{"message": "ok"})
}
