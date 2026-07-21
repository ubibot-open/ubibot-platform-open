package api_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

func TestDeviceActivatedFlagTracksActivationHistory(t *testing.T) {
	env := newTestEnv(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	rec, body := env.do(t, "GET", fmt.Sprintf("/api/admin/devices/%d", env.dev.ID), nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("get device failed: %d %v", rec.Code, body)
	}
	dev := body["device"].(map[string]interface{})
	if dev["activated"] != false {
		t.Fatalf("expected a freshly provisioned device to be unactivated, got %v", dev["activated"])
	}

	env.activateViaNonce(t)

	rec, body = env.do(t, "GET", fmt.Sprintf("/api/admin/devices/%d", env.dev.ID), nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("get device failed: %d %v", rec.Code, body)
	}
	dev = body["device"].(map[string]interface{})
	if dev["activated"] != true {
		t.Fatalf("expected activated=true after a successful activation, got %v", dev["activated"])
	}
}

func TestDisabledDeviceRejectsActivationAndReport(t *testing.T) {
	env := newTestEnv(t)
	deviceToken := env.activateViaNonce(t)

	// The device can report while enabled.
	rec, body := env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": 1000, "d": map[string]any{"x": 1}}},
	}, map[string]string{"X-IoT-Token": deviceToken})
	if rec.Code != 200 {
		t.Fatalf("expected report to succeed while enabled: %d %v", rec.Code, body)
	}

	if err := env.srv.Store.SetDeviceStatus(env.dev.ID, model.DeviceStatusDisabled); err != nil {
		t.Fatalf("disable device: %v", err)
	}

	// The same, still-unexpired token must now be rejected.
	rec, body = env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": 1001, "d": map[string]any{"x": 1}}},
	}, map[string]string{"X-IoT-Token": deviceToken})
	if rec.Code == 200 {
		t.Fatalf("expected report to be rejected once the device is disabled, got 200: %v", body)
	}

	// Config polling must also be rejected.
	pollReq := fmt.Sprintf("/api/v1/device/poll?did=%s", testSN)
	rec, body = env.do(t, "GET", pollReq, nil, map[string]string{"X-IoT-Token": deviceToken})
	if rec.Code == 200 {
		t.Fatalf("expected poll to be rejected once the device is disabled, got 200: %v", body)
	}

	// A fresh time-sync/activation attempt must also be rejected.
	rec, _ = env.do(t, "POST", "/api/v1/auth/time", map[string]any{
		"pid": testPID, "sn": testSN, "sign": env.sign(testPID, testSN),
	}, nil)
	if rec.Code == 200 {
		t.Fatalf("expected time-sync to be rejected for a disabled device")
	}
}

func TestOfflineSweepSkipsNeverActivatedDevice(t *testing.T) {
	env := newTestEnv(t)
	// env.dev is provisioned but never activated (newTestEnv creates it
	// directly via the store, bypassing the activation handshake).

	future := env.now.Add(1 * time.Hour)
	if err := env.srv.Store.OfflineSweep(future); err != nil {
		t.Fatalf("offline sweep: %v", err)
	}

	_, total, err := env.srv.Store.ListAlertEvents(store.AlertFilter{DeviceID: env.dev.ID}, 1, 20)
	if err != nil {
		t.Fatalf("list alert events: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected no offline alert for a never-activated device, got total=%d", total)
	}
}
