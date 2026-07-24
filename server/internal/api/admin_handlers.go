package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/auth"
	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

// --- request/response shapes -------------------------------------------
// These are this app's own admin REST API, not the device wire protocol —
// unlike internal/protocol, there's no external doc governing their
// shape, so they live next to the handlers that use them.

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in"`
	Username  string `json:"username"`
}

// deviceDTO is deliberately tiny per docs §6: a device only has an
// identity (pid/sn), a display name, an enable/disable status, and
// observed state (online/last-seen/created). There's no secret, source,
// activation flag, or per-device config to show anymore.
type deviceDTO struct {
	ID         uint   `json:"id"`
	PID        string `json:"pid"`
	SN         string `json:"sn"`
	Name       string `json:"name"`
	Status     int    `json:"status"`
	Online     bool   `json:"online"`
	LastSeenAt *int64 `json:"last_seen_at"`
	CreatedAt  int64  `json:"created_at"`
}

// toDeviceDTO's Online field uses the same rule (store.IsDeviceOnline) the
// offline-alert sweep does, so the device list/detail view and the alert
// center never disagree about which devices are up. now is the caller's
// s.Now() rather than time.Now() directly so this stays testable against
// a mocked clock.
func toDeviceDTO(d *model.Device, now time.Time) deviceDTO {
	dto := deviceDTO{
		ID:        d.ID,
		PID:       d.PID,
		SN:        d.SN,
		Name:      d.Name,
		Status:    d.Status,
		Online:    store.IsDeviceOnline(d, now),
		CreatedAt: d.CreatedAt.Unix(),
	}
	if d.LastSeenAt != nil {
		t := d.LastSeenAt.Unix()
		dto.LastSeenAt = &t
	}
	return dto
}

type recordDTO struct {
	Ts int64          `json:"ts"`
	D  map[string]any `json:"d"`
}

func paginationParams(r *http.Request) (page, pageSize int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ = strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return page, pageSize
}

// --- handlers ------------------------------------------------------------

// AdminLogin handles POST /api/admin/login.
func (s *Server) AdminLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil || req.Username == "" || req.Password == "" {
		adminErr(w, 400, "username and password are required")
		return
	}

	admin, err := s.Store.AdminByUsername(req.Username)
	if err != nil {
		adminErr(w, 401, "invalid username or password")
		return
	}
	if !auth.VerifyPassword(admin.PasswordHash, req.Password) {
		adminErr(w, 401, "invalid username or password")
		return
	}

	token, ttl, err := s.Store.IssueAdminSession(admin.ID)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}

	writeJSON(w, 200, loginResponse{Token: token, ExpiresIn: int64(ttl.Seconds()), Username: admin.Username})
}

// AdminMe handles GET /api/admin/me — lets the frontend show who's logged
// in without decoding anything client-side.
func (s *Server) AdminMe(w http.ResponseWriter, r *http.Request) {
	admin := currentAdmin(r)
	writeJSON(w, 200, map[string]any{"username": admin.Username})
}

// ListDevices handles GET /api/admin/devices.
func (s *Server) ListDevices(w http.ResponseWriter, r *http.Request) {
	page, pageSize := paginationParams(r)

	devices, total, err := s.Store.ListDevices(page, pageSize)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}

	list := make([]deviceDTO, 0, len(devices))
	for i := range devices {
		list = append(list, toDeviceDTO(&devices[i], s.Now()))
	}
	writeJSON(w, 200, map[string]any{"list": list, "total": total})
}

// dataWarehouseItemDTO is a deviceDTO plus that device's single most recent
// telemetry record (nil if it has never reported), for the "数据仓库" list's
// sensor-data preview column. Embedding deviceDTO flattens its fields into
// this one's JSON object (id/pid/sn/... alongside last_record).
type dataWarehouseItemDTO struct {
	deviceDTO
	LastRecord *recordDTO `json:"last_record"`
}

// ListDataWarehouse handles GET /api/admin/devices/data-warehouse — like
// ListDevices, annotated with each device's latest report, so the frontend
// can render a live sensor-data preview per row without an extra request
// per device. Every device in the table has reported at least once by
// construction (see store.GetOrCreateDeviceBySN), so unlike the old
// "activated devices only" filter, this is now just ListDevices plus the
// latest-record join.
func (s *Server) ListDataWarehouse(w http.ResponseWriter, r *http.Request) {
	page, pageSize := paginationParams(r)

	devices, total, err := s.Store.ListDevices(page, pageSize)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}

	ids := make([]uint, len(devices))
	for i := range devices {
		ids[i] = devices[i].ID
	}
	latest, err := s.Store.LatestRecordsByDevice(ids)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}

	now := s.Now()
	list := make([]dataWarehouseItemDTO, 0, len(devices))
	for i := range devices {
		item := dataWarehouseItemDTO{deviceDTO: toDeviceDTO(&devices[i], now)}
		if rec, ok := latest[devices[i].ID]; ok {
			var d map[string]any
			_ = json.Unmarshal([]byte(rec.Data), &d)
			item.LastRecord = &recordDTO{Ts: rec.Ts, D: d}
		}
		list = append(list, item)
	}
	writeJSON(w, 200, map[string]any{"list": list, "total": total})
}

// GetDevice handles GET /api/admin/devices/{id} — detail view with recent
// telemetry, enough for "后台能看" without a full historical query UI
// (that's GetDeviceRecords).
func (s *Server) GetDevice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}

	dev, err := s.Store.DeviceByID(uint(id))
	if errors.Is(err, store.ErrNotFound) {
		adminErr(w, 404, "device not found")
		return
	}
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}

	records, err := s.Store.RecentRecords(dev.ID, 20)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	recordDTOs := make([]recordDTO, 0, len(records))
	for _, rec := range records {
		var d map[string]any
		_ = json.Unmarshal([]byte(rec.Data), &d)
		recordDTOs = append(recordDTOs, recordDTO{Ts: rec.Ts, D: d})
	}

	writeJSON(w, 200, map[string]any{
		"device":  toDeviceDTO(dev, s.Now()),
		"records": recordDTOs,
	})
}

type renameDeviceRequest struct {
	Name string `json:"name"`
}

// RenameDevice handles PATCH /api/admin/devices/{id} — the only thing
// about a device an operator can configure after it auto-appears (see
// docs §6). An empty name is allowed (clears back to showing the SN in
// the frontend).
func (s *Server) RenameDevice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}

	var req renameDeviceRequest
	if err := decodeJSON(r, &req); err != nil {
		adminErr(w, 400, "malformed request body")
		return
	}

	if err := s.Store.RenameDevice(uint(id), req.Name); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "device.rename", "device", uint(id), req.Name)

	dev, err := s.Store.DeviceByID(uint(id))
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	writeJSON(w, 200, toDeviceDTO(dev, s.Now()))
}

type setStatusRequest struct {
	Status int `json:"status"`
}

// SetDeviceStatus handles POST /api/admin/devices/{id}/status — the
// enable/disable toggle. A disabled device is rejected by every
// device-facing endpoint (docs §6/§7, code 1103).
func (s *Server) SetDeviceStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}

	var req setStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		adminErr(w, 400, "status is required")
		return
	}
	if req.Status != model.DeviceStatusEnabled && req.Status != model.DeviceStatusDisabled {
		adminErr(w, 400, "invalid status")
		return
	}

	if err := s.Store.SetDeviceStatus(uint(id), req.Status); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "device.set_status", "device", uint(id), strconv.Itoa(req.Status))
	writeJSON(w, 200, map[string]any{"message": "ok"})
}

// DeleteDevice handles DELETE /api/admin/devices/{id} — permanently
// removes the device and every record that references it (see
// store.DeleteDevice). Irreversible; the frontend is expected to confirm
// with the operator before ever calling this.
func (s *Server) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}

	dev, err := s.Store.DeviceByID(uint(id))
	if errors.Is(err, store.ErrNotFound) {
		adminErr(w, 404, "device not found")
		return
	}
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}

	if err := s.Store.DeleteDevice(uint(id)); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "device.delete", "device", uint(id), dev.SN)
	writeJSON(w, 200, map[string]any{"message": "ok"})
}

// GetDeviceRecords handles GET /api/admin/devices/{id}/records?start=&end=
// — the "历史数据查询" page's backing endpoint. start/end are Unix
// seconds; omit either to leave that bound open.
func (s *Server) GetDeviceRecords(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	start, _ := strconv.ParseInt(r.URL.Query().Get("start"), 10, 64)
	end, _ := strconv.ParseInt(r.URL.Query().Get("end"), 10, 64)
	page, pageSize := paginationParams(r)

	records, total, err := s.Store.QueryRecords(uint(id), start, end, page, pageSize)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]recordDTO, 0, len(records))
	for _, rec := range records {
		var d map[string]any
		_ = json.Unmarshal([]byte(rec.Data), &d)
		list = append(list, recordDTO{Ts: rec.Ts, D: d})
	}
	writeJSON(w, 200, map[string]any{"list": list, "total": total})
}
