package api_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/store"
)

func TestNotificationCreatedOnAlertAndMarkRead(t *testing.T) {
	env := newTestEnv(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	env.do(t, "POST", fmt.Sprintf("/api/admin/devices/%d/alert-rules", env.dev.ID), map[string]any{
		"field": "field1", "op": ">", "threshold": 30,
	}, adminAuth)

	env.do(t, "POST", "/api/v1/data/report", report(testSN, env.now.Unix(), map[string]any{"field1": 35}), nil)

	rec, body := env.do(t, "GET", "/api/admin/notifications", nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("list notifications failed: %d %v", rec.Code, body)
	}
	list := body["list"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected 1 notification, got %v", list)
	}
	if body["unread"].(float64) != 1 {
		t.Fatalf("expected unread=1, got %v", body["unread"])
	}
	id := uint(list[0].(map[string]interface{})["id"].(float64))

	rec, _ = env.do(t, "POST", fmt.Sprintf("/api/admin/notifications/%d/read", id), nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("mark read failed: %d", rec.Code)
	}

	_, body = env.do(t, "GET", "/api/admin/notifications", nil, adminAuth)
	if body["unread"].(float64) != 0 {
		t.Fatalf("expected unread=0 after marking read, got %v", body["unread"])
	}
}

func TestOpenApiKeyAuth(t *testing.T) {
	env := newTestEnv(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	rec, body := env.do(t, "POST", "/api/admin/api-keys", map[string]any{"name": "集成测试"}, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("create api key failed: %d %v", rec.Code, body)
	}
	rawKey := body["raw_key"].(string)
	keyID := uint(body["key"].(map[string]interface{})["id"].(float64))

	rec, _ = env.do(t, "GET", "/api/open/v1/devices", nil, nil)
	if rec.Code != 401 {
		t.Fatalf("expected 401 without an api key, got %d", rec.Code)
	}

	rec, body = env.do(t, "GET", "/api/open/v1/devices", nil, map[string]string{"X-Api-Key": rawKey})
	if rec.Code != 200 {
		t.Fatalf("expected the open api call to succeed: %d %v", rec.Code, body)
	}
	if len(body["list"].([]interface{})) != 1 {
		t.Fatalf("expected 1 device, got %v", body["list"])
	}

	rec, _ = env.do(t, "POST", fmt.Sprintf("/api/admin/api-keys/%d/revoke", keyID), nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("revoke failed: %d", rec.Code)
	}

	rec, _ = env.do(t, "GET", "/api/open/v1/devices", nil, map[string]string{"X-Api-Key": rawKey})
	if rec.Code != 401 {
		t.Fatalf("expected a revoked key to be rejected, got %d", rec.Code)
	}
}

func TestDashboardSummaryAndTrends(t *testing.T) {
	env := newTestEnv(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	env.do(t, "POST", "/api/v1/data/report", report(testSN, env.now.Unix(), map[string]any{"field1": 20}), nil)

	rec, body := env.do(t, "GET", "/api/admin/dashboard/summary", nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("dashboard summary failed: %d %v", rec.Code, body)
	}
	if body["device_total"].(float64) != 1 {
		t.Fatalf("expected device_total=1, got %v", body)
	}
	if body["device_online"].(float64) != 1 {
		t.Fatalf("expected device_online=1 right after a report, got %v", body)
	}
	if body["today_records"].(float64) != 1 {
		t.Fatalf("expected today_records=1, got %v", body)
	}

	rec, body = env.do(t, "GET", "/api/admin/dashboard/trends", nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("dashboard trends failed: %d %v", rec.Code, body)
	}
	if _, ok := body["days"]; !ok {
		t.Fatalf("expected a days field, got %v", body)
	}
}

func TestDictEntryCrud(t *testing.T) {
	env := newTestEnv(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	rec, body := env.do(t, "POST", "/api/admin/dict", map[string]any{
		"type": "command_type", "key": "reboot", "label": "重启设备", "sort": 1,
	}, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("create dict entry failed: %d %v", rec.Code, body)
	}
	id := uint(body["id"].(float64))

	rec, body = env.do(t, "GET", "/api/admin/dict?type=command_type", nil, adminAuth)
	if rec.Code != 200 || len(body["list"].([]interface{})) != 1 {
		t.Fatalf("expected 1 dict entry, got %d %v", rec.Code, body)
	}

	rec, _ = env.do(t, "PATCH", fmt.Sprintf("/api/admin/dict/%d", id), map[string]any{"label": "重启", "sort": 2}, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("update dict entry failed: %d", rec.Code)
	}

	rec, _ = env.do(t, "DELETE", fmt.Sprintf("/api/admin/dict/%d", id), nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("delete dict entry failed: %d", rec.Code)
	}
}

func TestSystemParamAppliesRateLimitLive(t *testing.T) {
	env := newTestEnv(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	rec, body := env.do(t, "PATCH", "/api/admin/params/"+store.ParamRateLimitPerMinute, map[string]any{"value": "5"}, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("set param failed: %d %v", rec.Code, body)
	}

	limiter := env.srv.RateLimiter
	now := time.Now()
	for i := 0; i < 5; i++ {
		if !limiter.Allow("param-test-key", now) {
			t.Fatalf("request %d should have been allowed under the new limit of 5", i)
		}
	}
	if limiter.Allow("param-test-key", now) {
		t.Fatalf("6th request should have been blocked after lowering the limit to 5")
	}
}

func TestSystemMetricsEndpoint(t *testing.T) {
	env := newTestEnv(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	rec, body := env.do(t, "GET", "/api/admin/system/metrics", nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("system metrics failed: %d %v", rec.Code, body)
	}
	if _, ok := body["goroutines"]; !ok {
		t.Fatalf("expected a goroutines field, got %v", body)
	}
	if _, ok := body["device_total"]; !ok {
		t.Fatalf("expected a device_total field, got %v", body)
	}
}
