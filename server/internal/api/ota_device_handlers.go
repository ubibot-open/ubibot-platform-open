package api

import (
	"errors"
	"net/http"
	"os"
	"strconv"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

// OtaFirmwareDownload handles GET /api/v1/ota/firmware?fw=<id> (protocol
// §7.3) — token-authenticated, Range-request friendly so a device with
// limited RAM/flaky connectivity can pull the image in chunks and resume
// after a power loss. http.ServeContent already implements conditional
// GET and Range handling correctly, so this handler just needs to supply
// a ReadSeeker.
func (s *Server) OtaFirmwareDownload(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-IoT-Token")
	if token == "" {
		writeErr(w, protocol.CodeTokenInvalid, "missing token")
		return
	}
	dev, _, status, err := s.Store.ValidateDeviceToken(token)
	if err != nil {
		writeErr(w, protocol.CodeServerError, "internal error")
		return
	}
	switch status {
	case store.DeviceTokenNotFound:
		writeErr(w, protocol.CodeTokenInvalid, "token invalid")
		return
	case store.DeviceTokenExpired:
		writeErr(w, protocol.CodeTokenExpired, "token expired")
		return
	}
	if dev.Status != model.DeviceStatusEnabled {
		writeErr(w, protocol.CodeDeviceNotFound, "device disabled")
		return
	}

	fwID, convErr := strconv.Atoi(r.URL.Query().Get("fw"))
	if convErr != nil {
		writeErr(w, protocol.CodeMalformedBody, "missing or invalid fw id")
		return
	}
	fw, err := s.Store.FirmwareByID(uint(fwID))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeErr(w, protocol.CodeServerError, "internal error")
		return
	}
	// A firmware built for a different product shouldn't be servable to
	// this device even if it somehow learned the fw id — avoids one
	// compromised token being used to probe/download unrelated images.
	if fw.PID != dev.PID {
		http.NotFound(w, r)
		return
	}

	f, err := os.Open(fw.Path)
	if err != nil {
		writeErr(w, protocol.CodeServerError, "firmware file unavailable")
		return
	}
	defer f.Close()

	http.ServeContent(w, r, fw.Filename, fw.CreatedAt, f)
}
