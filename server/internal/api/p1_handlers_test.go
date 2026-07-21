package api_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/api"
	"github.com/ubibot/ubibot-platform-open/internal/auth"
	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

// createSuperAdmin seeds a super_admin role (permissions "*") and an admin
// account under it, returning the auth header for that account — the P1
// tests below mostly want "can this action succeed at all", not "does RBAC
// deny it", so they authenticate as the role that always passes.
func (e *testEnv) createSuperAdmin(t *testing.T, username, password string) map[string]string {
	t.Helper()
	role, err := e.srv.Store.CreateRole("超级管理员", model.RoleSuper, []string{"*"})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := e.srv.Store.CreateAdmin(username, hash, role.ID); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	_, body := e.do(t, "POST", "/api/admin/login", map[string]any{"username": username, "password": password}, nil)
	token, ok := body["token"].(string)
	if !ok {
		t.Fatalf("login did not return a token: %v", body)
	}
	return map[string]string{"Authorization": "Bearer " + token}
}

func TestProbeUpsertAckAndNakFlow(t *testing.T) {
	env := newTestEnv(t)
	deviceToken := env.activateViaNonce(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	// Configure a probe — this queues a set_probe(op=upsert) command and
	// records the probe as pending.
	rec, body := env.do(t, "POST", fmt.Sprintf("/api/admin/devices/%d/probes", env.dev.ID), map[string]any{
		"pid": "p1", "key": "soil_temp", "iface": "rs485", "proto": "modbus",
		"params": map[string]any{"addr": 1, "fc": 3, "reg": 0, "cnt": 1, "dtype": "int16", "scale": 0.1},
	}, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("upsert probe failed: %d %v", rec.Code, body)
	}
	probe := body["probe"].(map[string]interface{})
	if probe["status"] != model.ProbeStatusPending {
		t.Fatalf("expected pending probe, got %v", probe["status"])
	}
	cmd := body["command"].(map[string]interface{})
	cmdID := cmd["id"].(string)
	if cmd["type"] != "set_probe" {
		t.Fatalf("expected a set_probe command, got %v", cmd["type"])
	}

	// The device sees the command on its next report and acks it.
	rec, body = env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": 3000, "d": map[string]any{"x": 1}}},
	}, map[string]string{"X-IoT-Token": deviceToken})
	if rec.Code != 200 {
		t.Fatalf("report failed: %d %v", rec.Code, body)
	}
	cmds := body["cmd"].([]interface{})
	if len(cmds) != 1 || cmds[0].(map[string]interface{})["id"] != cmdID {
		t.Fatalf("expected the set_probe command to be delivered, got %v", body["cmd"])
	}

	env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": 3001, "d": map[string]any{"x": 2}}},
		"ack": []string{cmdID},
	}, map[string]string{"X-IoT-Token": deviceToken})

	rec, body = env.do(t, "GET", fmt.Sprintf("/api/admin/devices/%d/probes", env.dev.ID), nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("list probes failed: %d %v", rec.Code, body)
	}
	list := body["list"].([]interface{})
	if len(list) != 1 || list[0].(map[string]interface{})["status"] != model.ProbeStatusApplied {
		t.Fatalf("expected probe to be applied after ack, got %v", list)
	}

	// Remove the probe and nak the removal — it should be marked failed,
	// not silently deleted.
	rec, body = env.do(t, "DELETE", fmt.Sprintf("/api/admin/devices/%d/probes/p1", env.dev.ID), nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("remove probe failed: %d %v", rec.Code, body)
	}
	removeCmdID := body["command"].(map[string]interface{})["id"].(string)

	env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": 3002, "d": map[string]any{"x": 3}}},
		"nak": []map[string]any{{"id": removeCmdID, "c": 1, "m": "register busy"}},
	}, map[string]string{"X-IoT-Token": deviceToken})

	rec, body = env.do(t, "GET", fmt.Sprintf("/api/admin/devices/%d/probes", env.dev.ID), nil, adminAuth)
	list = body["list"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected the probe row to survive a nak'd removal, got %v", list)
	}
	entry := list[0].(map[string]interface{})
	if entry["status"] != model.ProbeStatusFailed || entry["last_error"] != "register busy" {
		t.Fatalf("expected failed status with nak reason recorded, got %v", entry)
	}
}

func TestDeviceRecordsQuery(t *testing.T) {
	env := newTestEnv(t)
	deviceToken := env.activateViaNonce(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	for _, ts := range []int64{1000, 2000, 3000} {
		env.do(t, "POST", "/api/v1/data/report", map[string]any{
			"did": testSN, "recs": []map[string]any{{"ts": ts, "d": map[string]any{"temperature": 20}}},
		}, map[string]string{"X-IoT-Token": deviceToken})
	}

	rec, body := env.do(t, "GET", fmt.Sprintf("/api/admin/devices/%d/records?start=1500&end=2500", env.dev.ID), nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("query records failed: %d %v", rec.Code, body)
	}
	list := body["list"].([]interface{})
	if len(list) != 1 || list[0].(map[string]interface{})["ts"].(float64) != 2000 {
		t.Fatalf("expected exactly the ts=2000 record within [1500,2500], got %v", list)
	}
}

func TestDeviceOnlineStatusReflectsLastSeen(t *testing.T) {
	env := newTestEnv(t)
	deviceToken := env.activateViaNonce(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": 1000, "d": map[string]any{"temperature": 20}}},
	}, map[string]string{"X-IoT-Token": deviceToken})

	rec, body := env.do(t, "GET", fmt.Sprintf("/api/admin/devices/%d", env.dev.ID), nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("get device failed: %d %v", rec.Code, body)
	}
	dev := body["device"].(map[string]interface{})
	if dev["online"] != true {
		t.Fatalf("expected device to be online right after a report, got %v", dev["online"])
	}

	// Move the test clock far past the offline grace period without another
	// report — the device should now read as offline.
	env.now = env.now.Add(1 * time.Hour)
	rec, body = env.do(t, "GET", fmt.Sprintf("/api/admin/devices/%d", env.dev.ID), nil, adminAuth)
	dev = body["device"].(map[string]interface{})
	if dev["online"] != false {
		t.Fatalf("expected device to be offline after the grace period elapsed, got %v", dev["online"])
	}
}

func TestThresholdAlertOpensAndAutoResolves(t *testing.T) {
	env := newTestEnv(t)
	deviceToken := env.activateViaNonce(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	rec, body := env.do(t, "POST", fmt.Sprintf("/api/admin/devices/%d/alert-rules", env.dev.ID), map[string]any{
		"field": "temperature", "op": ">", "threshold": 30,
	}, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("create alert rule failed: %d %v", rec.Code, body)
	}

	// A violating reading opens an alert event.
	env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": 1000, "d": map[string]any{"temperature": 35}}},
	}, map[string]string{"X-IoT-Token": deviceToken})

	rec, body = env.do(t, "GET", "/api/admin/alert-events?status=open", nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("list alert events failed: %d %v", rec.Code, body)
	}
	list := body["list"].([]interface{})
	if len(list) != 1 || list[0].(map[string]interface{})["type"] != model.AlertTypeThreshold {
		t.Fatalf("expected one open threshold alert, got %v", list)
	}

	// A subsequent non-violating reading auto-resolves it.
	env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": 1001, "d": map[string]any{"temperature": 20}}},
	}, map[string]string{"X-IoT-Token": deviceToken})

	rec, body = env.do(t, "GET", "/api/admin/alert-events?status=open", nil, adminAuth)
	list = body["list"].([]interface{})
	if len(list) != 0 {
		t.Fatalf("expected the alert to have auto-resolved, got %v", list)
	}
}

func TestOfflineSweepOpensAlert(t *testing.T) {
	env := newTestEnv(t)
	deviceToken := env.activateViaNonce(t)

	// Report once so the device has a LastSeenAt to go stale from, then
	// sweep far enough past it to cross the offline grace period.
	env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": 1000, "d": map[string]any{"x": 1}}},
	}, map[string]string{"X-IoT-Token": deviceToken})

	future := env.now.Add(1 * time.Hour)
	if err := env.srv.Store.OfflineSweep(future); err != nil {
		t.Fatalf("offline sweep: %v", err)
	}

	events, total, err := env.srv.Store.ListAlertEvents(store.AlertFilter{DeviceID: env.dev.ID, Status: model.AlertStatusOpen}, 1, 20)
	if err != nil {
		t.Fatalf("list alert events: %v", err)
	}
	if total != 1 || len(events) != 1 || events[0].Type != model.AlertTypeOffline {
		t.Fatalf("expected exactly one open offline alert after the sweep, got total=%d events=%v", total, events)
	}

	// Coming back online (a fresh report, still with the same live token) —
	// advance the mocked clock to when that report actually happens, so
	// TouchLastSeen records a LastSeenAt the next sweep will see as fresh.
	env.now = future
	env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": 2000, "d": map[string]any{"x": 1}}},
	}, map[string]string{"X-IoT-Token": deviceToken})
	if err := env.srv.Store.OfflineSweep(future); err != nil {
		t.Fatalf("offline sweep after recovery: %v", err)
	}
	_, total, err = env.srv.Store.ListAlertEvents(store.AlertFilter{DeviceID: env.dev.ID, Status: model.AlertStatusOpen}, 1, 20)
	if err != nil {
		t.Fatalf("list alert events: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected the offline alert to resolve once the device reported again, got total=%d", total)
	}
}

func TestRBACDeniesUnpermittedAction(t *testing.T) {
	env := newTestEnv(t)

	// A role with read-only device permissions cannot create a device.
	role, err := env.srv.Store.CreateRole("只读操作员", "readonly_op", []string{model.PermDeviceRead})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	hash, err := auth.HashPassword("pw12345")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := env.srv.Store.CreateAdmin("readonly", hash, role.ID); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	_, loginBody := env.do(t, "POST", "/api/admin/login", map[string]any{"username": "readonly", "password": "pw12345"}, nil)
	auth := map[string]string{"Authorization": "Bearer " + loginBody["token"].(string)}

	// Reading the device list is allowed.
	rec, _ := env.do(t, "GET", "/api/admin/devices", nil, auth)
	if rec.Code != 200 {
		t.Fatalf("expected device:read to permit listing devices, got %d", rec.Code)
	}

	// Creating a device (device:write) must be denied.
	rec, _ = env.do(t, "POST", "/api/admin/devices", map[string]any{
		"pid": "p", "sn": "sn-should-not-exist",
	}, auth)
	if rec.Code != 403 {
		t.Fatalf("expected device:write to be forbidden for a read-only role, got %d", rec.Code)
	}

	// System management (roles/users/audit) must also be denied.
	rec, _ = env.do(t, "GET", "/api/admin/roles", nil, auth)
	if rec.Code != 403 {
		t.Fatalf("expected system:manage to be forbidden for a read-only role, got %d", rec.Code)
	}
}

func TestIPRateLimiterBlocksAfterLimit(t *testing.T) {
	limiter := api.NewIPLimiter(3, time.Minute)
	now := time.Now()
	for i := 0; i < 3; i++ {
		if !limiter.Allow("1.2.3.4", now) {
			t.Fatalf("request %d should have been allowed within the limit", i)
		}
	}
	if limiter.Allow("1.2.3.4", now) {
		t.Fatalf("4th request within the same window should have been blocked")
	}
	// A different key has its own independent budget.
	if !limiter.Allow("5.6.7.8", now) {
		t.Fatalf("a different client IP should not share the exhausted budget")
	}
	// Once the window rolls over, the original key gets a fresh budget.
	if !limiter.Allow("1.2.3.4", now.Add(time.Minute+time.Second)) {
		t.Fatalf("expected the limiter to reset once the window elapsed")
	}
}
