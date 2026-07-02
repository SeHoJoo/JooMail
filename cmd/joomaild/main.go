package main

import (
	"log"
	"net/http"
	"os"

	"github.com/SeHoJoo/JooMail/internal/httpapi"
)

func main() {
	addr := os.Getenv("JOOMAIL_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	server := httpapi.NewServer(httpapi.MockStore())
	if staticDir := os.Getenv("JOOMAIL_STATIC_DIR"); staticDir != "" {
		server = httpapi.WithStaticFiles(server, staticDir)
	}

	log.Printf("joomail backend listening on http://%s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatal(err)
	}
}
