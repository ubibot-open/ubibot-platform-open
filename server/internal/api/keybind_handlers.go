package api

import (
	"errors"
	"net/http"

	"github.com/ubibot/ubibot-platform-open/internal/auth"
	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

// PublicKey handles GET /api/v1/auth/public-key (docs §4.1) — lets a
// self-registration provisioning tool fetch this platform's RSA public key
// so it can encrypt a device's real secret before submitting it via
// BindDeviceKey. Deliberately unauthenticated and not rate-limited beyond
// the shared device-facing limiter: the key is public by definition, and
// handing it out reveals nothing about any device.
func (s *Server) PublicKey(w http.ResponseWriter, r *http.Request) {
	if s.ServerKeyPair == nil {
		writeErr(w, protocol.CodeServerError, "server key pair not configured")
		return
	}
	writeJSON(w, 200, protocol.PublicKeyResponse{C: protocol.CodeOK, PublicKeyPEM: s.ServerKeyPair.PublicPEM})
}

// BindDeviceKey handles POST /api/v1/auth/bind-key (docs §4.1) — the
// provisioning-tool side of self-registration. A device that self-
// registered (model.DeviceSourceSelfRegistered) was auto-created with
// Secret="" the moment its SN showed up unrecognized (see
// maybeAutoRegisterDevice), because there is no way to tell a
// pre-manufactured device what secret the platform picked. This endpoint
// is how that device's real, factory-set secret finally reaches the
// platform instead: a provisioning tool reads it directly off the
// hardware (serial/BLE), encrypts it against this platform's public key
// (see PublicKey), and submits {sn, secret} here. A successful bind both
// records the secret and — if the device was still Pending — completes
// its activation, exactly like api.SetDeviceSecret does from the admin
// side.
func (s *Server) BindDeviceKey(w http.ResponseWriter, r *http.Request) {
	var req protocol.BindKeyRequest
	if err := decodeJSON(r, &req); err != nil || req.SN == "" || req.Secret == "" {
		writeErr(w, protocol.CodeMalformedBody, "malformed request body")
		return
	}
	if s.ServerKeyPair == nil {
		writeErr(w, protocol.CodeServerError, "server key pair not configured")
		return
	}

	dev, err := s.Store.DeviceBySN(req.SN)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, protocol.CodeKeyBindFailed, "device not found or not eligible for key binding")
		return
	}
	if err != nil {
		writeErr(w, protocol.CodeServerError, "internal error")
		return
	}
	if dev.Source != model.DeviceSourceSelfRegistered {
		// Manually-created devices already got their secret via
		// CreateDevice's one-time reveal — this channel is only for the
		// self-registration path that has no other way to set one.
		writeErr(w, protocol.CodeKeyBindFailed, "device not found or not eligible for key binding")
		return
	}

	secret, err := auth.DecryptDeviceSecret(s.ServerKeyPair.PrivateKey, req.Secret)
	if err != nil {
		writeErr(w, protocol.CodeKeyBindFailed, "key decrypt failed")
		return
	}

	if err := s.Store.SetDeviceSecret(dev.ID, secret); err != nil {
		writeErr(w, protocol.CodeServerError, "internal error")
		return
	}

	writeJSON(w, 200, protocol.BindKeyResponse{C: protocol.CodeOK, M: "ok"})
}
