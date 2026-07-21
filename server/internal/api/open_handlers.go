package api

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// OpenListDevices handles GET /api/open/v1/devices — the read-only
// third-party integration surface (see RequireApiKey), deliberately a
// much smaller DTO than the admin API's (no secret, no fe/ci/ui
// internals a third party has no use for).
func (s *Server) OpenListDevices(w http.ResponseWriter, r *http.Request) {
	page, pageSize := paginationParams(r)
	devices, total, err := s.Store.ListDevices(page, pageSize)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]map[string]any, 0, len(devices))
	for i := range devices {
		dto := toDeviceDTO(&devices[i], s.Now())
		list = append(list, map[string]any{
			"id": dto.ID, "sn": dto.SN, "name": dto.Name, "online": dto.Online, "last_seen_at": dto.LastSeenAt,
		})
	}
	writeJSON(w, 200, map[string]any{"list": list, "total": total})
}

// OpenGetDeviceRecords handles GET /api/open/v1/devices/{id}/records —
// the open-API equivalent of the admin history query, same
// start/end/page params.
func (s *Server) OpenGetDeviceRecords(w http.ResponseWriter, r *http.Request) {
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
