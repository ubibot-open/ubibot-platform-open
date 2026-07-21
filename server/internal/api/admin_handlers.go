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

type deviceDTO struct {
	ID         uint     `json:"id"`
	PID        string   `json:"pid"`
	SN         string   `json:"sn"`
	Name       string   `json:"name"`
	Status     int      `json:"status"`
	Online     bool     `json:"online"`
	CI         int      `json:"ci"`
	UI         int      `json:"ui"`
	FE         []string `json:"fe"`
	LastSeenAt *int64   `json:"last_seen_at"`
	CreatedAt  int64    `json:"created_at"`
	Secret     string   `json:"secret,omitempty"` // only populated by CreateDevice
}

// toDeviceDTO's Online field uses the same rule (store.IsDeviceOnline) the
// offline-alert sweep does, so the device list/detail view and the alert
// center never disagree about which devices are up. now is the caller's
// s.Now() rather than time.Now() directly so this stays testable against
// a mocked clock, same as the rest of the device-facing time logic.
func toDeviceDTO(d *model.Device, now time.Time) deviceDTO {
	var fe []string
	if d.FE != "" {
		_ = json.Unmarshal([]byte(d.FE), &fe)
	}
	dto := deviceDTO{
		ID:        d.ID,
		PID:       d.PID,
		SN:        d.SN,
		Name:      d.Name,
		Status:    d.Status,
		Online:    store.IsDeviceOnline(d, now),
		CI:        d.CI,
		UI:        d.UI,
		FE:        fe,
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

type commandDTO struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Args       map[string]any `json:"args,omitempty"`
	Status     string         `json:"status"`
	NakMessage string         `json:"nak_message,omitempty"`
	CreatedAt  int64          `json:"created_at"`
}

func toCommandDTO(cmd *model.DeviceCommand) commandDTO {
	dto := commandDTO{
		ID:         cmd.CmdID,
		Type:       cmd.Type,
		Status:     cmd.Status,
		NakMessage: cmd.NakMessage,
		CreatedAt:  cmd.CreatedAt.Unix(),
	}
	if cmd.Args != "" {
		var a map[string]any
		if err := json.Unmarshal([]byte(cmd.Args), &a); err == nil {
			dto.Args = a
		}
	}
	return dto
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

type createDeviceRequest struct {
	PID    string `json:"pid"`
	SN     string `json:"sn"`
	Secret string `json:"secret"`
	Name   string `json:"name"`
}

// CreateDevice handles POST /api/admin/devices. This is the only place a
// device's secret is ever shown back — write it down / flash it now, the
// API won't return it again.
func (s *Server) CreateDevice(w http.ResponseWriter, r *http.Request) {
	var req createDeviceRequest
	if err := decodeJSON(r, &req); err != nil || req.PID == "" || req.SN == "" {
		adminErr(w, 400, "pid and sn are required")
		return
	}

	secret := req.Secret
	if secret == "" {
		var err error
		secret, err = auth.NewDeviceSecret()
		if err != nil {
			adminErr(w, 500, "internal error")
			return
		}
	}

	dev, err := s.Store.CreateDevice(req.PID, req.SN, secret, req.Name)
	if err != nil {
		adminErr(w, 400, "device already exists or invalid input")
		return
	}

	s.audit(r, "device.create", "device", dev.ID, dev.SN)

	dto := toDeviceDTO(dev, s.Now())
	dto.Secret = secret
	writeJSON(w, 200, dto)
}

// GetDevice handles GET /api/admin/devices/{id} — detail view with recent
// telemetry and recent command history, enough for "后台能看" without a
// full historical query UI (that's a later slice).
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

	commands, _, err := s.Store.ListCommands(dev.ID, 1, 20)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	commandDTOs := make([]commandDTO, 0, len(commands))
	for i := range commands {
		commandDTOs = append(commandDTOs, toCommandDTO(&commands[i]))
	}

	writeJSON(w, 200, map[string]any{
		"device":   toDeviceDTO(dev, s.Now()),
		"records":  recordDTOs,
		"commands": commandDTOs,
	})
}

type updateConfigRequest struct {
	CI int      `json:"ci"`
	UI int      `json:"ui"`
	FE []string `json:"fe"`
}

// UpdateDeviceConfig handles PATCH /api/admin/devices/{id}/config. The
// change reaches the device on its next report/poll via the existing
// cfg-diff push in store.ProcessReport — this endpoint only edits the
// desired state.
func (s *Server) UpdateDeviceConfig(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}

	var req updateConfigRequest
	if err := decodeJSON(r, &req); err != nil || req.CI <= 0 || req.UI <= 0 {
		adminErr(w, 400, "ci and ui are required")
		return
	}

	if err := s.Store.SetDeviceConfig(uint(id), req.CI, req.UI, req.FE); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "device.update_config", "device", uint(id), "")
	writeJSON(w, 200, map[string]any{"message": "ok"})
}

type setStatusRequest struct {
	Status int `json:"status"`
}

// SetDeviceStatus handles POST /api/admin/devices/{id}/status.
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

type dispatchCommandRequest struct {
	Type string         `json:"type"`
	Args map[string]any `json:"args"`
}

// DispatchCommand handles POST /api/admin/devices/{id}/commands — this is
// the "手动下发一条指令" entry point: it queues a row that
// store.PendingCommands will attach to the device's next report/poll
// response, and that the admin device-detail view can then watch flip to
// acked (or nacked) once the device processes it.
func (s *Server) DispatchCommand(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}

	if _, err := s.Store.DeviceByID(uint(id)); errors.Is(err, store.ErrNotFound) {
		adminErr(w, 404, "device not found")
		return
	}

	var req dispatchCommandRequest
	if err := decodeJSON(r, &req); err != nil || req.Type == "" {
		adminErr(w, 400, "type is required")
		return
	}

	cmd, err := s.Store.QueueCommand(uint(id), req.Type, req.Args)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "command.dispatch", "device", uint(id), req.Type)
	writeJSON(w, 200, toCommandDTO(cmd))
}

// ListCommands handles GET /api/admin/devices/{id}/commands.
func (s *Server) ListCommands(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	page, pageSize := paginationParams(r)

	commands, total, err := s.Store.ListCommands(uint(id), page, pageSize)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]commandDTO, 0, len(commands))
	for i := range commands {
		list = append(list, toCommandDTO(&commands[i]))
	}
	writeJSON(w, 200, map[string]any{"list": list, "total": total})
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

// ListAllCommands handles GET /api/admin/commands — cross-device command
// history for the "指令管理" page, filterable by device_id/status/type.
// (ListCommands above is the same data scoped to one device's detail
// view.)
func (s *Server) ListAllCommands(w http.ResponseWriter, r *http.Request) {
	page, pageSize := paginationParams(r)
	deviceID, _ := strconv.Atoi(r.URL.Query().Get("device_id"))

	f := store.CommandFilter{
		DeviceID: uint(deviceID),
		Status:   r.URL.Query().Get("status"),
		Type:     r.URL.Query().Get("type"),
	}
	commands, total, err := s.Store.ListAllCommands(f, page, pageSize)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}

	deviceNames := make(map[uint]string)
	list := make([]map[string]any, 0, len(commands))
	for i := range commands {
		dto := toCommandDTO(&commands[i])
		name, ok := deviceNames[commands[i].DeviceID]
		if !ok {
			if dev, err := s.Store.DeviceByID(commands[i].DeviceID); err == nil {
				name = dev.Name
				if name == "" {
					name = dev.SN
				}
			}
			deviceNames[commands[i].DeviceID] = name
		}
		list = append(list, map[string]any{
			"id": dto.ID, "type": dto.Type, "args": dto.Args, "status": dto.Status,
			"nak_message": dto.NakMessage, "created_at": dto.CreatedAt,
			"device_id": commands[i].DeviceID, "device_name": name,
		})
	}
	writeJSON(w, 200, map[string]any{"list": list, "total": total})
}
