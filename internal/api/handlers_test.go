package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ubibot/ubibot-platform-open/internal/api"
	"github.com/ubibot/ubibot-platform-open/internal/auth"
	"github.com/ubibot/ubibot-platform-open/internal/protocol"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const (
	testPID    = "ubibot_open_dev_v1"
	testSN     = "sn_ws1_20001_1"
	testSecret = "test-secret"
)

// testEnv bundles a router with a device already provisioned and a clock
// the test controls, so nonce/token expiry and the activation time window
// can be exercised deterministically instead of racing the wall clock.
type testEnv struct {
	router http.Handler
	srv    *api.Server
	now    time.Time
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	st := store.New()
	st.RegisterDevice(store.Device{PID: testPID, SN: testSN, Secret: testSecret})

	srv := api.NewServer(st)
	env := &testEnv{srv: srv, now: time.Unix(1_700_000_000, 0)}
	srv.Now = func() time.Time { return env.now }
	env.router = api.NewRouter(srv)
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
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)

	var parsed map[string]interface{}
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("response is not JSON: %v (body=%s)", err, w.Body.String())
		}
	}
	return w, parsed
}

func (e *testEnv) timeSync(t *testing.T) (w *httptest.ResponseRecorder, body map[string]interface{}) {
	t.Helper()
	sign := auth.Sign(testSecret, testPID, testSN)
	return e.do(t, http.MethodPost, "/api/v1/auth/time", map[string]string{
		"pid": testPID, "sn": testSN, "sign": sign,
	}, nil)
}

func (e *testEnv) activateWithNonce(t *testing.T) (token string, exp int64) {
	t.Helper()
	_, tsBody := e.timeSync(t)
	ts := int64(tsBody["t"].(float64))
	n := tsBody["n"].(string)

	sign := auth.Sign(testSecret, testPID, testSN, strconv.FormatInt(ts, 10), n)
	w, body := e.do(t, http.MethodPost, "/api/v1/auth/activate", map[string]interface{}{
		"pid": testPID, "sn": testSN, "ts": ts, "n": n, "sign": sign,
	}, nil)
	if w.Code != 200 {
		t.Fatalf("activate failed: %d %v", w.Code, body)
	}
	return body["token"].(string), int64(body["exp"].(float64))
}

func TestTimeSync_Success(t *testing.T) {
	env := newTestEnv(t)
	w, body := env.timeSync(t)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if body["c"].(float64) != protocol.CodeOK {
		t.Fatalf("c = %v, want 0", body["c"])
	}
	if int64(body["t"].(float64)) != env.now.Unix() {
		t.Fatalf("t = %v, want %d", body["t"], env.now.Unix())
	}
	if n, _ := body["n"].(string); n == "" {
		t.Fatal("n (nonce) missing from response")
	}
}

func TestTimeSync_BadSign(t *testing.T) {
	env := newTestEnv(t)
	w, body := env.do(t, http.MethodPost, "/api/v1/auth/time", map[string]string{
		"pid": testPID, "sn": testSN, "sign": "0000",
	}, nil)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if body["c"].(float64) != protocol.CodeSignMismatch {
		t.Fatalf("c = %v, want %d", body["c"], protocol.CodeSignMismatch)
	}
}

func TestTimeSync_UnknownDevice(t *testing.T) {
	env := newTestEnv(t)
	sign := auth.Sign("whatever-secret", testPID, "sn-does-not-exist")
	w, body := env.do(t, http.MethodPost, "/api/v1/auth/time", map[string]string{
		"pid": testPID, "sn": "sn-does-not-exist", "sign": sign,
	}, nil)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	// Same code as a bad signature: unknown-device must not be distinguishable
	// from wrong-signature, or the endpoint becomes a device-enumeration oracle.
	if body["c"].(float64) != protocol.CodeSignMismatch {
		t.Fatalf("c = %v, want %d", body["c"], protocol.CodeSignMismatch)
	}
}

func TestActivate_WithNonce_Success(t *testing.T) {
	env := newTestEnv(t)
	token, exp := env.activateWithNonce(t)

	if token == "" {
		t.Fatal("token missing")
	}
	if exp != int64(auth.TokenTTL.Seconds()) {
		t.Fatalf("exp = %d, want %d", exp, int64(auth.TokenTTL.Seconds()))
	}
}

func TestActivate_NonceReuse_Rejected(t *testing.T) {
	env := newTestEnv(t)
	_, tsBody := env.timeSync(t)
	ts := int64(tsBody["t"].(float64))
	n := tsBody["n"].(string)
	sign := auth.Sign(testSecret, testPID, testSN, strconv.FormatInt(ts, 10), n)

	req := map[string]interface{}{"pid": testPID, "sn": testSN, "ts": ts, "n": n, "sign": sign}

	w1, _ := env.do(t, http.MethodPost, "/api/v1/auth/activate", req, nil)
	if w1.Code != 200 {
		t.Fatalf("first activation failed: %d", w1.Code)
	}

	w2, body2 := env.do(t, http.MethodPost, "/api/v1/auth/activate", req, nil)
	if w2.Code != 400 {
		t.Fatalf("replayed activation status = %d, want 400", w2.Code)
	}
	if body2["c"].(float64) != protocol.CodeSignMismatch {
		t.Fatalf("c = %v, want %d", body2["c"], protocol.CodeSignMismatch)
	}
}

func TestActivate_LocalClock_NoNonce_Success(t *testing.T) {
	env := newTestEnv(t)
	ts := env.now.Unix()
	sign := auth.Sign(testSecret, testPID, testSN, strconv.FormatInt(ts, 10), "")

	w, body := env.do(t, http.MethodPost, "/api/v1/auth/activate", map[string]interface{}{
		"pid": testPID, "sn": testSN, "ts": ts, "sign": sign,
	}, nil)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200 (body=%v)", w.Code, body)
	}
	if body["token"].(string) == "" {
		t.Fatal("token missing")
	}
}

func TestActivate_LocalClock_OutOfWindow(t *testing.T) {
	env := newTestEnv(t)
	ts := env.now.Add(-10 * time.Minute).Unix() // outside the ±5 minute window
	sign := auth.Sign(testSecret, testPID, testSN, strconv.FormatInt(ts, 10), "")

	w, body := env.do(t, http.MethodPost, "/api/v1/auth/activate", map[string]interface{}{
		"pid": testPID, "sn": testSN, "ts": ts, "sign": sign,
	}, nil)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if body["c"].(float64) != protocol.CodeSignMismatch {
		t.Fatalf("c = %v, want %d", body["c"], protocol.CodeSignMismatch)
	}
}

func TestReport_ConfigSentOnlyOnChange(t *testing.T) {
	env := newTestEnv(t)
	token, _ := env.activateWithNonce(t)

	env.srv.Store.SetConfig(testSN, protocol.Config{CI: 15, UI: 300, FE: []string{"temperature"}})

	reportBody := map[string]interface{}{
		"did": testSN,
		"recs": []map[string]interface{}{
			{"ts": env.now.Unix(), "d": map[string]interface{}{"temperature": 25.6}},
		},
	}
	headers := map[string]string{"X-IoT-Token": token}

	w1, body1 := env.do(t, http.MethodPost, "/api/v1/data/report", reportBody, headers)
	if w1.Code != 200 {
		t.Fatalf("first report status = %d, want 200 (body=%v)", w1.Code, body1)
	}
	cfg1, ok := body1["cfg"].(map[string]interface{})
	if !ok {
		t.Fatal("first report response missing cfg after a config change")
	}
	if int(cfg1["ci"].(float64)) != 15 || int(cfg1["ui"].(float64)) != 300 {
		t.Fatalf("cfg = %v, want ci=15 ui=300", cfg1)
	}
	if got := w1.Header().Get("X-Token-Expires-In"); got == "" {
		t.Fatal("X-Token-Expires-In header missing")
	}

	w2, body2 := env.do(t, http.MethodPost, "/api/v1/data/report", reportBody, headers)
	if w2.Code != 200 {
		t.Fatalf("second report status = %d, want 200", w2.Code)
	}
	if _, present := body2["cfg"]; present {
		t.Fatalf("second report should omit unchanged cfg, got %v", body2["cfg"])
	}
}

func TestReport_CommandDeliveryAndAck(t *testing.T) {
	env := newTestEnv(t)
	token, _ := env.activateWithNonce(t)

	cmd := env.srv.Store.QueueCommand(testSN, "reboot", nil)

	reportBody := func(ack []string) map[string]interface{} {
		body := map[string]interface{}{
			"did": testSN,
			"recs": []map[string]interface{}{
				{"ts": env.now.Unix(), "d": map[string]interface{}{"temperature": 25.6}},
			},
		}
		if ack != nil {
			body["ack"] = ack
		}
		return body
	}
	headers := map[string]string{"X-IoT-Token": token}

	w1, body1 := env.do(t, http.MethodPost, "/api/v1/data/report", reportBody(nil), headers)
	if w1.Code != 200 {
		t.Fatalf("status = %d, want 200", w1.Code)
	}
	cmds, ok := body1["cmd"].([]interface{})
	if !ok || len(cmds) != 1 {
		t.Fatalf("cmd = %v, want one queued command", body1["cmd"])
	}
	first := cmds[0].(map[string]interface{})
	if first["id"].(string) != cmd.ID || first["tp"].(string) != "reboot" {
		t.Fatalf("cmd[0] = %v, want id=%s tp=reboot", first, cmd.ID)
	}

	w2, body2 := env.do(t, http.MethodPost, "/api/v1/data/report", reportBody([]string{cmd.ID}), headers)
	if w2.Code != 200 {
		t.Fatalf("status = %d, want 200", w2.Code)
	}
	if _, present := body2["cmd"]; present {
		t.Fatalf("cmd should be empty after ack, got %v", body2["cmd"])
	}
}

func TestReport_MultipleRecsBatched(t *testing.T) {
	env := newTestEnv(t)
	token, _ := env.activateWithNonce(t)

	ts1, ts2 := env.now.Unix(), env.now.Add(10*time.Minute).Unix()
	reportBody := map[string]interface{}{
		"did": testSN,
		"recs": []map[string]interface{}{
			{"ts": ts1, "d": map[string]interface{}{"temperature": 25.6, "humidity": 60.2}},
			{"ts": ts2, "d": map[string]interface{}{"temperature": 25.8, "npk": map[string]interface{}{"n": 120, "p": 100, "k": 100}}},
		},
	}

	w, body := env.do(t, http.MethodPost, "/api/v1/data/report", reportBody, map[string]string{"X-IoT-Token": token})
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200 (body=%v)", w.Code, body)
	}
	if !env.srv.Store.Seen(testSN, ts1) || !env.srv.Store.Seen(testSN, ts2) {
		t.Fatal("both batched records should be recorded")
	}
}

func TestReport_MissingToken(t *testing.T) {
	env := newTestEnv(t)
	w, body := env.do(t, http.MethodPost, "/api/v1/data/report", map[string]interface{}{
		"did":  testSN,
		"recs": []map[string]interface{}{{"ts": env.now.Unix(), "d": map[string]interface{}{"temperature": 1}}},
	}, nil)

	if w.Code != 401 {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if body["c"].(float64) != protocol.CodeTokenInvalid {
		t.Fatalf("c = %v, want %d", body["c"], protocol.CodeTokenInvalid)
	}
}

func TestReport_ExpiredToken(t *testing.T) {
	env := newTestEnv(t)
	token, _ := env.activateWithNonce(t)

	env.now = env.now.Add(25 * time.Hour) // past the 24h token TTL

	w, body := env.do(t, http.MethodPost, "/api/v1/data/report", map[string]interface{}{
		"did":  testSN,
		"recs": []map[string]interface{}{{"ts": env.now.Unix(), "d": map[string]interface{}{"temperature": 1}}},
	}, map[string]string{"X-IoT-Token": token})

	if w.Code != 401 {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if body["c"].(float64) != protocol.CodeTokenExpired {
		t.Fatalf("c = %v, want %d", body["c"], protocol.CodeTokenExpired)
	}
}

func TestReport_DeviceMismatch(t *testing.T) {
	env := newTestEnv(t)
	token, _ := env.activateWithNonce(t)

	w, body := env.do(t, http.MethodPost, "/api/v1/data/report", map[string]interface{}{
		"did":  "some-other-device",
		"recs": []map[string]interface{}{{"ts": env.now.Unix(), "d": map[string]interface{}{"temperature": 1}}},
	}, map[string]string{"X-IoT-Token": token})

	if w.Code != 401 {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if body["c"].(float64) != protocol.CodeDeviceNotFound {
		t.Fatalf("c = %v, want %d", body["c"], protocol.CodeDeviceNotFound)
	}
}
