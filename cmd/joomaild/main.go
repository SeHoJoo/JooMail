package main

import (
	"log"
	"net/http"
	"os"

	"github.com/SeHoJoo/JooMail/internal/httpapi"
)

func main() {
	config := httpapi.LoadConfig()
	addr := os.Getenv("JOOMAIL_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	if config.SessionSecret == "" {
		log.Print("warning: JOOMAIL_SESSION_SECRET is empty; /api/login will fail closed until it is set")
	}

	server := httpapi.NewServerWithConfig(httpapi.MockStore(), config)
	if staticDir := os.Getenv("JOOMAIL_STATIC_DIR"); staticDir != "" {
		server = httpapi.WithStaticFiles(server, staticDir)
	}

	log.Printf("joomail backend listening on http://%s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatal(err)
	}
}
