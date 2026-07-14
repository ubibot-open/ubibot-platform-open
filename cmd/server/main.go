// Command server runs the UbiBot device-facing HTTP API described in
// docs/UbiBot开放平台硬件通信协议.docx.
package main

import (
	"log"
	"os"

	"github.com/ubibot/ubibot-platform-open/internal/api"
	"github.com/ubibot/ubibot-platform-open/internal/store"
)

func main() {
	st := store.New()

	// Demo device so the endpoints are usable out of the box; a real
	// deployment would provision devices out of band (factory flashing +
	// a device registry), which is out of scope for this protocol server.
	st.RegisterDevice(store.Device{
		PID:    "ubibot_open_dev_v1",
		SN:     "sn_ws1_20001_1",
		Secret: "demo-secret-change-me",
	})

	srv := api.NewServer(st)
	r := api.NewRouter(srv)

	addr := os.Getenv("UBIBOT_LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("ubibot device API listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
