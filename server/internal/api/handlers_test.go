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
	testPID    = "ubibot_open_dev_v1"
	testSN     = "sn_ws1_20001_1"
	testSecret = "test-secret"
)

// testEnv bundles a router with a device already provisioned and a clock
// the test controls, so nonce/token expiry and the activation time window
// can be exercised deterministically instead of racing the wall clock.
// Each test gets its own in-memory database (via a unique DSN) so tests
// can run in parallel without stepping on each other's rows.
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

	dev, err := st.CreateDevice(testPID, testSN, testSecret, "test device")
	if err != nil {
		t.Fatalf("create device: %v", err)
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

func (e *testEnv) sign(parts ...string) string {
	return auth.Sign(testSecret, parts...)
}

// activateViaNonce runs the time-sync + activate handshake and returns the
// issued session token.
func (e *testEnv) activateViaNonce(t *testing.T) string {
	t.Helper()

	_, timeBody := e.do(t, "POST", "/api/v1/auth/time", map[string]any{
		"pid": testPID, "sn": testSN, "sign": e.sign(testPID, testSN),
	}, nil)
	n := timeBody["n"].(string)
	ts := int64(timeBody["t"].(float64))

	_, actBody := e.do(t, "POST", "/api/v1/auth/activate", map[string]any{
		"pid": testPID, "sn": testSN, "ts": ts, "n": n,
		"sign": e.sign(testPID, testSN, auth.FormatTs(ts), n),
	}, nil)
	return actBody["token"].(string)
}

func TestTimeSyncAndActivate_NonceReplayRejected(t *testing.T) {
	env := newTestEnv(t)

	rec, body := env.do(t, "POST", "/api/v1/auth/time", map[string]any{
		"pid": testPID, "sn": testSN, "sign": env.sign(testPID, testSN),
	}, nil)
	if rec.Code != 200 || body["c"].(float64) != 0 {
		t.Fatalf("time sync failed: %d %v", rec.Code, body)
	}
	n := body["n"].(string)
	ts := int64(body["t"].(float64))
	sign := env.sign(testPID, testSN, auth.FormatTs(ts), n)

	rec, body = env.do(t, "POST", "/api/v1/auth/activate", map[string]any{
		"pid": testPID, "sn": testSN, "ts": ts, "n": n, "sign": sign,
	}, nil)
	if rec.Code != 200 || body["token"] == "" {
		t.Fatalf("activate failed: %d %v", rec.Code, body)
	}

	// Replaying the exact same signed request must fail: the nonce was
	// already consumed.
	rec, body = env.do(t, "POST", "/api/v1/auth/activate", map[string]any{
		"pid": testPID, "sn": testSN, "ts": ts, "n": n, "sign": sign,
	}, nil)
	if rec.Code == 200 {
		t.Fatalf("expected nonce replay to be rejected, got 200: %v", body)
	}
}

func TestActivate_LocalClockPath_MonotonicTsRequired(t *testing.T) {
	env := newTestEnv(t)
	ts := env.now.Unix()
	sign := env.sign(testPID, testSN, auth.FormatTs(ts), "")

	rec, body := env.do(t, "POST", "/api/v1/auth/activate", map[string]any{
		"pid": testPID, "sn": testSN, "ts": ts, "sign": sign,
	}, nil)
	if rec.Code != 200 {
		t.Fatalf("first activation should succeed: %d %v", rec.Code, body)
	}

	// Same ts again (a replayed request within the ±5min window) must be
	// rejected — this is the monotonic guard closing the window-replay gap.
	rec, _ = env.do(t, "POST", "/api/v1/auth/activate", map[string]any{
		"pid": testPID, "sn": testSN, "ts": ts, "sign": sign,
	}, nil)
	if rec.Code == 200 {
		t.Fatalf("expected replayed ts to be rejected")
	}

	// An older ts must also be rejected, even if still within the window.
	olderTs := ts - 1
	rec, _ = env.do(t, "POST", "/api/v1/auth/activate", map[string]any{
		"pid": testPID, "sn": testSN, "ts": olderTs,
		"sign": env.sign(testPID, testSN, auth.FormatTs(olderTs), ""),
	}, nil)
	if rec.Code == 200 {
		t.Fatalf("expected older ts to be rejected")
	}

	// A strictly greater ts must succeed.
	newerTs := ts + 1
	rec, body = env.do(t, "POST", "/api/v1/auth/activate", map[string]any{
		"pid": testPID, "sn": testSN, "ts": newerTs,
		"sign": env.sign(testPID, testSN, auth.FormatTs(newerTs), ""),
	}, nil)
	if rec.Code != 200 {
		t.Fatalf("expected advancing ts to succeed: %d %v", rec.Code, body)
	}
}

func TestReport_DedupAndDidBinding(t *testing.T) {
	env := newTestEnv(t)
	token := env.activateViaNonce(t)

	rec, body := env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did":  testSN,
		"recs": []map[string]any{{"ts": 1000, "d": map[string]any{"temperature": 25.6}}},
	}, map[string]string{"X-IoT-Token": token})
	if rec.Code != 200 || body["c"].(float64) != 0 {
		t.Fatalf("report failed: %d %v", rec.Code, body)
	}

	// Same (did, ts) again with a different value must not overwrite —
	// the unique index + ON CONFLICT DO NOTHING makes this a no-op.
	env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did":  testSN,
		"recs": []map[string]any{{"ts": 1000, "d": map[string]any{"temperature": 999}}},
	}, map[string]string{"X-IoT-Token": token})

	records, err := env.srv.Store.RecentRecords(env.dev.ID, 10)
	if err != nil {
		t.Fatalf("recent records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected exactly 1 deduped record, got %d", len(records))
	}
	var d map[string]any
	_ = json.Unmarshal([]byte(records[0].Data), &d)
	if d["temperature"].(float64) != 25.6 {
		t.Fatalf("expected the first value to win, got %v", d["temperature"])
	}

	// A report whose did doesn't match the token's device must be rejected
	// (protects against a token holder reporting as an arbitrary did).
	rec, body = env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did":  "some-other-device",
		"recs": []map[string]any{{"ts": 1001, "d": map[string]any{"temperature": 1}}},
	}, map[string]string{"X-IoT-Token": token})
	if rec.Code == 200 {
		t.Fatalf("expected did/token mismatch to be rejected: %v", body)
	}
}

func TestReport_MissingOrInvalidToken(t *testing.T) {
	env := newTestEnv(t)

	rec, _ := env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": 1, "d": map[string]any{}}},
	}, nil)
	if rec.Code != 401 {
		t.Fatalf("expected 401 for missing token, got %d", rec.Code)
	}

	rec, _ = env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": 1, "d": map[string]any{}}},
	}, map[string]string{"X-IoT-Token": "not-a-real-token"})
	if rec.Code != 401 {
		t.Fatalf("expected 401 for invalid token, got %d", rec.Code)
	}
}

func TestAdminLoginAndCommandDispatchFlow(t *testing.T) {
	env := newTestEnv(t)
	deviceToken := env.activateViaNonce(t)

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

	// List devices shows the provisioned device.
	rec, body = env.do(t, "GET", "/api/admin/devices", nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("list devices failed: %d %v", rec.Code, body)
	}
	list := body["list"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected 1 device, got %d", len(list))
	}

	// Dispatch a command — this is the "手动下发一条指令" entry point.
	rec, body = env.do(t, "POST", fmt.Sprintf("/api/admin/devices/%d/commands", env.dev.ID),
		map[string]any{"type": "reboot"}, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("dispatch command failed: %d %v", rec.Code, body)
	}
	cmdID := body["id"].(string)
	if body["status"].(string) != model.CommandStatusPending {
		t.Fatalf("expected freshly dispatched command to be pending, got %v", body["status"])
	}

	// The device picks up the pending command on its next report.
	rec, body = env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did":  testSN,
		"recs": []map[string]any{{"ts": 2000, "d": map[string]any{"temperature": 25.6}}},
	}, map[string]string{"X-IoT-Token": deviceToken})
	if rec.Code != 200 {
		t.Fatalf("report failed: %d %v", rec.Code, body)
	}
	cmds := body["cmd"].([]interface{})
	if len(cmds) != 1 || cmds[0].(map[string]interface{})["id"] != cmdID {
		t.Fatalf("expected the pending command to be delivered, got %v", body["cmd"])
	}

	// The device acks it on a later report.
	rec, _ = env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did":  testSN,
		"recs": []map[string]any{{"ts": 2001, "d": map[string]any{"temperature": 25.7}}},
		"ack":  []string{cmdID},
	}, map[string]string{"X-IoT-Token": deviceToken})
	if rec.Code != 200 {
		t.Fatalf("ack report failed: %d", rec.Code)
	}

	// The admin can see it flip to acked.
	rec, body = env.do(t, "GET", fmt.Sprintf("/api/admin/devices/%d", env.dev.ID), nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("get device failed: %d %v", rec.Code, body)
	}
	commands := body["commands"].([]interface{})
	if len(commands) != 1 || commands[0].(map[string]interface{})["status"] != model.CommandStatusAcked {
		t.Fatalf("expected command to be acked, got %v", commands)
	}
}
