package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/api"
	"github.com/ubibot/ubibot-platform-open/internal/auth"
	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

const (
	testPID = "ubibot_open_dev_v1"
	testSN  = "sn_ws1_20001_1"
)

// testEnv bundles a router with a device already provisioned (via the same
// GetOrCreateDeviceBySN path a real first report takes) and a clock the
// test controls, so the ±5min report window and offline-grace assertions
// can be exercised deterministically instead of racing the wall clock. Each
// test gets its own in-memory database (via a unique DSN) so tests can run
// in parallel without stepping on each other's rows.
type testEnv struct {
	router http.Handler
	srv    *api.Server
	now    time.Time
	dev    *model.Device
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	dev, _, err := st.GetOrCreateDeviceBySN(testPID, testSN)
	if err != nil {
		t.Fatalf("provision device: %v", err)
	}

	srv := api.NewServer(st)
	env := &testEnv{srv: srv, now: time.Unix(1_700_000_000, 0), dev: dev}
	srv.Now = func() time.Time { return env.now }
	env.router = api.NewRouter(srv, nil, false)
	return env
}

func (e *testEnv) do(t *testing.T, method, path string, body interface{}, headers map[string]string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)

	var parsed map[string]interface{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("decode response %q: %v", rec.Body.String(), err)
		}
	}
	return rec, parsed
}

// report is a small helper building a docs-§4-shaped report body: one
// payload with the given ts and feed.
func report(sn string, ts int64, feed map[string]any) map[string]any {
	return map[string]any{
		"pid": testPID, "sn": sn, "ts": ts,
		"payloads": []map[string]any{{"ts": ts, "feed": feed}},
	}
}

func TestTimeSync_ReturnsServerTime(t *testing.T) {
	env := newTestEnv(t)

	rec, body := env.do(t, "POST", "/api/v1/auth/time", map[string]any{
		"pid": testPID, "sn": testSN,
	}, nil)
	if rec.Code != 200 || body["c"].(float64) != 0 {
		t.Fatalf("time sync failed: %d %v", rec.Code, body)
	}
	if int64(body["t"].(float64)) != env.now.Unix() {
		t.Fatalf("expected server time %d, got %v", env.now.Unix(), body["t"])
	}
}

func TestTimeSync_RejectsMalformedBody(t *testing.T) {
	env := newTestEnv(t)

	rec, body := env.do(t, "POST", "/api/v1/auth/time", map[string]any{"pid": testPID}, nil)
	if rec.Code != 400 || body["c"].(float64) != 1003 {
		t.Fatalf("expected malformed-body rejection, got %d %v", rec.Code, body)
	}
}

func TestReport_AutoCreatesUnseenDeviceAndDedupesByTs(t *testing.T) {
	env := newTestEnv(t)
	const newSN = "sn_new_device_1"

	rec, body := env.do(t, "POST", "/api/v1/data/report",
		report(newSN, env.now.Unix(), map[string]any{"field1": 25.6}), nil)
	if rec.Code != 200 || body["c"].(float64) != 0 {
		t.Fatalf("report failed: %d %v", rec.Code, body)
	}

	dev, err := env.srv.Store.DeviceBySN(newSN)
	if err != nil {
		t.Fatalf("expected the unseen SN to have been auto-created: %v", err)
	}

	// Same (device, ts) again with a different value must not overwrite —
	// the unique index + ON CONFLICT DO NOTHING makes this a no-op.
	env.do(t, "POST", "/api/v1/data/report",
		report(newSN, env.now.Unix(), map[string]any{"field1": 999}), nil)

	records, err := env.srv.Store.RecentRecords(dev.ID, 10)
	if err != nil {
		t.Fatalf("recent records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected exactly 1 deduped record, got %d", len(records))
	}
	var d map[string]any
	_ = json.Unmarshal([]byte(records[0].Data), &d)
	if d["field1"].(float64) != 25.6 {
		t.Fatalf("expected the first value to win, got %v", d["field1"])
	}
}

func TestReport_RejectsMalformedBody(t *testing.T) {
	env := newTestEnv(t)

	rec, body := env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"pid": testPID, "sn": testSN,
	}, nil)
	if rec.Code != 400 || body["c"].(float64) != 1003 {
		t.Fatalf("expected malformed-body rejection for a missing ts/payloads, got %d %v", rec.Code, body)
	}
}

func TestReport_RejectsTimestampOutsideWindow(t *testing.T) {
	env := newTestEnv(t)

	future := env.now.Add(10 * time.Minute).Unix()
	rec, body := env.do(t, "POST", "/api/v1/data/report",
		report(testSN, future, map[string]any{"field1": 20}), nil)
	if rec.Code != 400 || body["c"].(float64) != 1002 {
		t.Fatalf("expected timestamp-out-of-window rejection, got %d %v", rec.Code, body)
	}
}

func TestAdminLoginAndDeviceListFlow(t *testing.T) {
	env := newTestEnv(t)

	role, err := env.srv.Store.CreateRole("超级管理员", model.RoleSuper, []string{"*"})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	hash, err := auth.HashPassword("s3cret-pw")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := env.srv.Store.CreateAdmin("admin", hash, role.ID); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	// Wrong password rejected.
	rec, _ := env.do(t, "POST", "/api/admin/login", map[string]any{"username": "admin", "password": "wrong"}, nil)
	if rec.Code != 401 {
		t.Fatalf("expected wrong password to be rejected, got %d", rec.Code)
	}

	// Correct login issues a bearer token.
	rec, body := env.do(t, "POST", "/api/admin/login", map[string]any{"username": "admin", "password": "s3cret-pw"}, nil)
	if rec.Code != 200 {
		t.Fatalf("admin login failed: %d %v", rec.Code, body)
	}
	adminToken := body["token"].(string)
	adminAuth := map[string]string{"Authorization": "Bearer " + adminToken}

	// Protected endpoints reject requests with no/invalid session.
	rec, _ = env.do(t, "GET", "/api/admin/devices", nil, nil)
	if rec.Code != 401 {
		t.Fatalf("expected 401 without session, got %d", rec.Code)
	}

	// List devices shows the seeded device.
	rec, body = env.do(t, "GET", "/api/admin/devices", nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("list devices failed: %d %v", rec.Code, body)
	}
	list := body["list"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected 1 device, got %d", len(list))
	}

	// Rename it — the only per-device config left (docs §6).
	rec, body = env.do(t, "PATCH", fmt.Sprintf("/api/admin/devices/%d", env.dev.ID),
		map[string]any{"name": "客厅传感器"}, adminAuth)
	if rec.Code != 200 || body["name"] != "客厅传感器" {
		t.Fatalf("rename device failed: %d %v", rec.Code, body)
	}

	// Delete it.
	rec, _ = env.do(t, "DELETE", fmt.Sprintf("/api/admin/devices/%d", env.dev.ID), nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("delete device failed: %d", rec.Code)
	}
	rec, _ = env.do(t, "GET", fmt.Sprintf("/api/admin/devices/%d", env.dev.ID), nil, adminAuth)
	if rec.Code != 404 {
		t.Fatalf("expected deleted device to 404, got %d", rec.Code)
	}
}
