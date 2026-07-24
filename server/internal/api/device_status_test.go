package api_test

import (
	"testing"

	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

func TestDisabledDeviceRejectsReport(t *testing.T) {
	env := newTestEnv(t)

	// The device can report while enabled.
	rec, body := env.do(t, "POST", "/api/v1/data/report", report(testSN, env.now.Unix(), map[string]any{"field1": 1}), nil)
	if rec.Code != 200 {
		t.Fatalf("expected report to succeed while enabled: %d %v", rec.Code, body)
	}

	if err := env.srv.Store.SetDeviceStatus(env.dev.ID, model.DeviceStatusDisabled); err != nil {
		t.Fatalf("disable device: %v", err)
	}

	// A subsequent report must now be rejected (docs §7, code 1103) — there
	// is no token to have expired, this is purely a live status check.
	rec, body = env.do(t, "POST", "/api/v1/data/report", report(testSN, env.now.Unix()+1, map[string]any{"field1": 1}), nil)
	if rec.Code != 401 || body["c"].(float64) != 1103 {
		t.Fatalf("expected 401/1103 once the device is disabled, got %d %v", rec.Code, body)
	}
}

// TestOfflineSweepAlertsNeverReportedDevice pins down an intentional edge
// case of the simplified provisioning model (see store.OfflineSweep's
// comment): a device row can exist with no LastSeenAt at all if it was
// created outside of a report (e.g. cmd/server's seedDemoDevice at
// startup) — unlike the old activation-flag model, there's no longer a
// "provisioned but never activated" state that the sweep skips, so such a
// device is swept as offline just like any other quiet device.
func TestOfflineSweepAlertsNeverReportedDevice(t *testing.T) {
	env := newTestEnv(t)
	// env.dev is provisioned via GetOrCreateDeviceBySN directly (see
	// newTestEnv), bypassing any report, so it has no LastSeenAt yet.

	if err := env.srv.Store.OfflineSweep(env.now); err != nil {
		t.Fatalf("offline sweep: %v", err)
	}

	_, total, err := env.srv.Store.ListAlertEvents(store.AlertFilter{DeviceID: env.dev.ID}, 1, 20)
	if err != nil {
		t.Fatalf("list alert events: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected a never-reported device to be swept as offline, got total=%d", total)
	}
}

var _ = time.Minute // keep time imported for readability of future clock-based additions
