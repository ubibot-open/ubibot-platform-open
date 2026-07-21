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

	"github.com/ubibot/ubibot-platform-open/internal/api"
	"github.com/ubibot/ubibot-platform-open/internal/auth"
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
	r := api.NewRouter(srv)

	addr := os.Getenv("UBIBOT_LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("ubibot API listening on %s (db: %s)", addr, dbPath)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}

// bootstrapAdmin creates a default admin account on first run so there is
// always a way to log in. The password is either read from
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
	if _, err := st.CreateAdmin(username, hash); err != nil {
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
