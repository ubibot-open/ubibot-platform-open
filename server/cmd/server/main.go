// Command server runs the UbiBot device-facing HTTP API described in
// docs/UbiBot开放平台硬件通信协议.md, plus the minimal admin API (login,
// device management, command dispatch) that sits on the same store.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ubibot/ubibot-platform-open/internal/api"
	"github.com/ubibot/ubibot-platform-open/internal/auth"
	"github.com/ubibot/ubibot-platform-open/internal/model"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

func main() {
	dbPath := os.Getenv("UBIBOT_DB_PATH")
	if dbPath == "" {
		dbPath = "./data/ubibot.db"
	}
	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("create db directory: %v", err)
		}
	}

	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}

	if err := bootstrapAdmin(st); err != nil {
		log.Fatalf("bootstrap admin: %v", err)
	}
	if err := seedDemoDevice(st); err != nil {
		log.Fatalf("seed demo device: %v", err)
	}

	srv := api.NewServer(st)
	srv.DBPath = dbPath
	dataDir := filepath.Dir(dbPath)
	srv.FirmwareDir = filepath.Join(dataDir, "firmware")
	srv.FileDir = filepath.Join(dataDir, "files")

	if err := seedDefaultParams(st); err != nil {
		log.Fatalf("seed default params: %v", err)
	}
	applyStoredParams(srv, st)

	r := api.NewRouter(srv)

	// Offline-alert detection is the absence of a report, so nothing about
	// receiving one can trigger it — a periodic sweep is the only way to
	// notice. Runs independently of request traffic for the life of the
	// process.
	go runOfflineSweepLoop(st, 30*time.Second)

	// Scheduled command dispatch (定时任务) — checked more often than the
	// offline sweep since interval-based tasks can be as short as a minute.
	go runScheduledTaskLoop(st, 15*time.Second)

	addr := os.Getenv("UBIBOT_LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("ubibot API listening on %s (db: %s)", addr, dbPath)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}

func runOfflineSweepLoop(st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := st.OfflineSweep(time.Now()); err != nil {
			log.Printf("offline sweep error: %v", err)
		}
	}
}

func runScheduledTaskLoop(st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := st.RunDueScheduledTasks(time.Now()); err != nil {
			log.Printf("scheduled task run error: %v", err)
		}
	}
}

// seedDefaultParams writes the whitelist of runtime-tunable system
// parameters (see internal/store/param.go, internal/api/param_handlers.go)
// on first run only — an admin who has already customized a value should
// never have it silently reset back to default by a restart.
func seedDefaultParams(st *store.Store) error {
	defaults := map[string]struct {
		value string
		desc  string
	}{
		store.ParamRateLimitPerMinute: {"120", "设备侧接口每IP每分钟请求上限"},
		store.ParamOfflineGraceMinute: {"2", "离线判定的最小宽限时间（分钟）"},
	}
	for key, d := range defaults {
		if _, ok := st.GetParam(key); ok {
			continue
		}
		if _, err := st.SetParam(key, d.value, d.desc); err != nil {
			return err
		}
	}
	return nil
}

// applyStoredParams pushes every persisted system parameter into live
// server state at startup — the DB is the source of truth, this just
// catches it up after a restart (see Server.ApplyParam for which keys
// actually do anything).
func applyStoredParams(srv *api.Server, st *store.Store) {
	params, err := st.ListParams()
	if err != nil {
		log.Printf("load system params: %v", err)
		return
	}
	for _, p := range params {
		srv.ApplyParam(p.Key, p.Value)
	}
}

// bootstrapAdmin creates a default super-admin role and account on first
// run so there is always a way to log in. The password is either read from
// UBIBOT_ADMIN_PASSWORD (set this in production) or generated and printed
// once — there is no other way to recover it afterward short of resetting
// the admin_users table.
func bootstrapAdmin(st *store.Store) error {
	n, err := st.CountAdmins()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	role, err := st.RoleByCode(model.RoleSuper)
	if errors.Is(err, store.ErrNotFound) {
		role, err = st.CreateRole("超级管理员", model.RoleSuper, []string{"*"})
	}
	if err != nil {
		return err
	}

	username := "admin"
	password := os.Getenv("UBIBOT_ADMIN_PASSWORD")
	generated := password == ""
	if generated {
		buf := make([]byte, 9)
		if _, err := rand.Read(buf); err != nil {
			return err
		}
		password = hex.EncodeToString(buf)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if _, err := st.CreateAdmin(username, hash, role.ID); err != nil {
		return err
	}

	if generated {
		log.Printf("no admin account found — created %q with a generated password: %s", username, password)
		log.Printf("this password is only shown once; set UBIBOT_ADMIN_PASSWORD to control it on next first run")
	}
	return nil
}

// seedDemoDevice provisions the same demo triple the in-memory reference
// server used to hardcode, but only once — so the API is usable out of the
// box without every restart re-registering it as if nothing had happened.
func seedDemoDevice(st *store.Store) error {
	const sn = "sn_ws1_20001_1"
	_, err := st.DeviceBySN(sn)
	if err == nil {
		return nil // already provisioned
	}
	if !errors.Is(err, store.ErrNotFound) {
		return err
	}

	if _, err := st.CreateDevice("ubibot_open_dev_v1", sn, "demo-secret-change-me", "示例设备"); err != nil {
		return err
	}
	log.Printf("seeded demo device sn=%s (secret: demo-secret-change-me)", sn)
	return nil
}
