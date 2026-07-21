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

// maxFirmwareUploadBytes bounds a single firmware upload — generous for
// any microcontroller-class image, tight enough to stop someone using the
// endpoint to fill the disk.
const maxFirmwareUploadBytes = 64 << 20 // 64MB

type firmwareDTO struct {
	ID        uint   `json:"id"`
	PID       string `json:"pid"`
	Version   string `json:"version"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	HasSig    bool   `json:"has_sig"`
	CreatedAt int64  `json:"created_at"`
}

func toFirmwareDTO(f *model.Firmware) firmwareDTO {
	return firmwareDTO{
		ID: f.ID, PID: f.PID, Version: f.Version, Filename: f.Filename,
		Size: f.Size, SHA256: f.SHA256, HasSig: f.Signature != "", CreatedAt: f.CreatedAt.Unix(),
	}
}

// UploadFirmware handles POST /api/admin/firmware (multipart/form-data:
// pid, version, signature[optional], file) — this is where a build
// artifact becomes something devices can be told to download (protocol
// §7.3's cmd.a.url points back at OtaFirmwareDownload for this row's id).
func (s *Server) UploadFirmware(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFirmwareUploadBytes)
	if err := r.ParseMultipartForm(maxFirmwareUploadBytes); err != nil {
		adminErr(w, 400, "invalid upload (file too large or malformed)")
		return
	}

	pid := r.FormValue("pid")
	version := r.FormValue("version")
	signature := r.FormValue("signature")
	if pid == "" || version == "" {
		adminErr(w, 400, "pid and version are required")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		adminErr(w, 400, "file is required")
		return
	}
	defer file.Close()

	if err := os.MkdirAll(s.FirmwareDir, 0o755); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	destName := fmt.Sprintf("%d_%s_%s", time.Now().UnixNano(), pid, header.Filename)
	destPath := filepath.Join(s.FirmwareDir, destName)

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
		adminErr(w, 500, "failed to store firmware file")
		return
	}

	fw, err := s.Store.CreateFirmware(pid, version, header.Filename, destPath, size, hex.EncodeToString(hasher.Sum(nil)), signature)
	if err != nil {
		_ = os.Remove(destPath)
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "firmware.upload", "firmware", fw.ID, pid+" "+version)
	writeJSON(w, 200, toFirmwareDTO(fw))
}

// ListFirmware handles GET /api/admin/firmware.
func (s *Server) ListFirmware(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListFirmware()
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]firmwareDTO, 0, len(rows))
	for i := range rows {
		list = append(list, toFirmwareDTO(&rows[i]))
	}
	writeJSON(w, 200, map[string]any{"list": list})
}

// DeleteFirmware handles DELETE /api/admin/firmware/{id}.
func (s *Server) DeleteFirmware(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	fw, err := s.Store.FirmwareByID(uint(id))
	if errors.Is(err, store.ErrNotFound) {
		adminErr(w, 404, "firmware not found")
		return
	}
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	if err := s.Store.DeleteFirmware(uint(id)); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	_ = os.Remove(fw.Path)
	s.audit(r, "firmware.delete", "firmware", uint(id), fw.Version)
	writeJSON(w, 200, map[string]any{"message": "ok"})
}

type deviceOtaDTO struct {
	FirmwareID uint   `json:"firmware_id"`
	Version    string `json:"version"`
	State      string `json:"state"`
	Progress   int    `json:"progress"`
	LastError  string `json:"last_error,omitempty"`
}

// GetDeviceOTA handles GET /api/admin/devices/{id}/ota.
func (s *Server) GetDeviceOTA(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	ota, err := s.Store.DeviceOTAByDevice(uint(id))
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, 200, map[string]any{"ota": nil})
		return
	}
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	writeJSON(w, 200, map[string]any{"ota": deviceOtaDTO{
		FirmwareID: ota.FirmwareID, Version: ota.Version, State: ota.State,
		Progress: ota.Progress, LastError: ota.LastError,
	}})
}

type dispatchOtaRequest struct {
	FirmwareID uint `json:"firmware_id"`
	Force      bool `json:"force"`
}

// DispatchDeviceOTA handles POST /api/admin/devices/{id}/ota — queues an
// ota(action=start) command (protocol §7.3) pointing the device at
// OtaFirmwareDownload for the chosen firmware row.
func (s *Server) DispatchDeviceOTA(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	if _, err := s.Store.DeviceByID(uint(id)); errors.Is(err, store.ErrNotFound) {
		adminErr(w, 404, "device not found")
		return
	}

	var req dispatchOtaRequest
	if err := decodeJSON(r, &req); err != nil || req.FirmwareID == 0 {
		adminErr(w, 400, "firmware_id is required")
		return
	}
	fw, err := s.Store.FirmwareByID(req.FirmwareID)
	if errors.Is(err, store.ErrNotFound) {
		adminErr(w, 404, "firmware not found")
		return
	}
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}

	downloadURL := fmt.Sprintf("/api/v1/ota/firmware?fw=%d", fw.ID)
	cmd, err := s.Store.DispatchOTA(uint(id), fw, downloadURL, req.Force)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "ota.dispatch", "device", uint(id), fw.Version)
	writeJSON(w, 200, toCommandDTO(cmd))
}

// CancelDeviceOTA handles POST /api/admin/devices/{id}/ota/cancel.
func (s *Server) CancelDeviceOTA(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	cmd, err := s.Store.CancelOTA(uint(id))
	if errors.Is(err, store.ErrNotFound) {
		adminErr(w, 404, "no ota task in progress for this device")
		return
	}
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "ota.cancel", "device", uint(id), "")
	writeJSON(w, 200, toCommandDTO(cmd))
}
