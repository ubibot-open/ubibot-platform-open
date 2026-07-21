package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/store"
)

// doMultipart is p2's variant of testEnv.do for the one admin endpoint
// that isn't plain JSON — firmware/file upload, which needs a real
// multipart/form-data body.
func (e *testEnv) doMultipart(
	t *testing.T, method, path string,
	fields map[string]string, fileField, fileName string, fileContent []byte,
	headers map[string]string,
) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	if fileField != "" {
		fw, err := mw.CreateFormFile(fileField, fileName)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := fw.Write(fileContent); err != nil {
			t.Fatalf("write file content: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
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

func TestOtaUploadDispatchDownloadAndAckFlow(t *testing.T) {
	env := newTestEnv(t)
	env.srv.FirmwareDir = t.TempDir()
	deviceToken := env.activateViaNonce(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	firmwareBytes := []byte("firmware-image-bytes-v1.0.1")
	rec, body := env.doMultipart(t, "POST", "/api/admin/firmware",
		map[string]string{"pid": testPID, "version": "1.0.1"},
		"file", "app.bin", firmwareBytes, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("upload firmware failed: %d %v", rec.Code, body)
	}
	fwID := uint(body["id"].(float64))
	if body["sha256"] == "" || body["size"].(float64) != float64(len(firmwareBytes)) {
		t.Fatalf("expected sha256/size to be computed from the uploaded file, got %v", body)
	}

	rec, body = env.do(t, "POST", fmt.Sprintf("/api/admin/devices/%d/ota", env.dev.ID),
		map[string]any{"firmware_id": fwID}, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("dispatch ota failed: %d %v", rec.Code, body)
	}
	cmdID := body["id"].(string)
	if body["type"] != "ota" {
		t.Fatalf("expected an ota command, got %v", body["type"])
	}

	// The device picks up the command on its next report.
	rec, body = env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": 5000, "d": map[string]any{"x": 1}}},
	}, map[string]string{"X-IoT-Token": deviceToken})
	if rec.Code != 200 {
		t.Fatalf("report failed: %d %v", rec.Code, body)
	}
	cmds := body["cmd"].([]interface{})
	if len(cmds) != 1 {
		t.Fatalf("expected the ota command to be delivered, got %v", body["cmd"])
	}
	args := cmds[0].(map[string]interface{})["a"].(map[string]interface{})
	downloadURL := args["url"].(string)

	// The device downloads the firmware, exercising Range/resume support.
	dlReq := httptest.NewRequest("GET", downloadURL, nil)
	dlReq.Header.Set("X-IoT-Token", deviceToken)
	dlReq.Header.Set("Range", "bytes=9-")
	dlRec := httptest.NewRecorder()
	env.router.ServeHTTP(dlRec, dlReq)
	if dlRec.Code != 206 {
		t.Fatalf("expected 206 partial content for a ranged download, got %d: %s", dlRec.Code, dlRec.Body.String())
	}
	if dlRec.Body.String() != string(firmwareBytes[9:]) {
		t.Fatalf("unexpected partial content: %q", dlRec.Body.String())
	}

	// Progress reports update the tracked OTA state.
	env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did":  testSN,
		"recs": []map[string]any{{"ts": 5001, "d": map[string]any{"x": 1}}},
		"ota":  map[string]any{"id": cmdID, "version": "1.0.1", "state": "flashing", "progress": 80},
	}, map[string]string{"X-IoT-Token": deviceToken})

	rec, body = env.do(t, "GET", fmt.Sprintf("/api/admin/devices/%d/ota", env.dev.ID), nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("get device ota failed: %d %v", rec.Code, body)
	}
	otaStatus := body["ota"].(map[string]interface{})
	if otaStatus["state"] != "flashing" || otaStatus["progress"].(float64) != 80 {
		t.Fatalf("expected state=flashing progress=80, got %v", otaStatus)
	}

	// Acking the cmd (post-reboot self-check handshake) finalizes success.
	env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did":  testSN,
		"recs": []map[string]any{{"ts": 5002, "d": map[string]any{"x": 1}}},
		"ack":  []string{cmdID},
	}, map[string]string{"X-IoT-Token": deviceToken})

	rec, body = env.do(t, "GET", fmt.Sprintf("/api/admin/devices/%d/ota", env.dev.ID), nil, adminAuth)
	otaStatus = body["ota"].(map[string]interface{})
	if otaStatus["state"] != "success" {
		t.Fatalf("expected state=success after ack, got %v", otaStatus)
	}

	// A success notification should have landed in 消息中心.
	rec, body = env.do(t, "GET", "/api/admin/notifications", nil, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("list notifications failed: %d %v", rec.Code, body)
	}
	found := false
	for _, n := range body["list"].([]interface{}) {
		if n.(map[string]interface{})["type"] == "ota" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an ota notification, got %v", body["list"])
	}
}

func TestNotificationCreatedOnAlertAndMarkRead(t *testing.T) {
	env := newTestEnv(t)
	deviceToken := env.activateViaNonce(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	env.do(t, "POST", fmt.Sprintf("/api/admin/devices/%d/alert-rules", env.dev.ID), map[string]any{
		"field": "temperature", "op": ">", "threshold": 30,
	}, adminAuth)

	env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": 1000, "d": map[string]any{"temperature": 35}}},
	}, map[string]string{"X-IoT-Token": deviceToken})

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

func TestScheduledTaskIntervalDispatchesCommand(t *testing.T) {
	env := newTestEnv(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	rec, body := env.do(t, "POST", "/api/admin/scheduled-tasks", map[string]any{
		"name": "每分钟健康检查", "device_id": env.dev.ID, "cmd_type": "ping",
		"schedule_type": "interval", "interval_seconds": 60, "enabled": true,
	}, adminAuth)
	if rec.Code != 200 {
		t.Fatalf("create scheduled task failed: %d %v", rec.Code, body)
	}

	// Scheduled-task timing runs off the real wall clock (cmd/server's
	// ticker calls time.Now(), not the device-protocol's mocked env.now),
	// so drive it forward from time.Now() too rather than env.now.
	future := time.Now().Add(2 * time.Minute)
	if err := env.srv.Store.RunDueScheduledTasks(future); err != nil {
		t.Fatalf("run due scheduled tasks: %v", err)
	}

	cmds, total, err := env.srv.Store.ListCommands(env.dev.ID, 1, 20)
	if err != nil {
		t.Fatalf("list commands: %v", err)
	}
	if total != 1 || cmds[0].Type != "ping" {
		t.Fatalf("expected the scheduled ping command to have been queued, got total=%d cmds=%v", total, cmds)
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
	deviceToken := env.activateViaNonce(t)
	adminAuth := env.createSuperAdmin(t, "admin", "s3cret-pw")

	env.do(t, "POST", "/api/v1/data/report", map[string]any{
		"did": testSN, "recs": []map[string]any{{"ts": env.now.Unix(), "d": map[string]any{"temperature": 20}}},
	}, map[string]string{"X-IoT-Token": deviceToken})

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
