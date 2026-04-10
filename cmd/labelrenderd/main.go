package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/jjcinaz/panel2/labelserver"
)

func main() {
	var err error
	var addr string
	var templateRoot string
	var handler *labelserver.Handler
	var mux *http.ServeMux
	var server *http.Server

	flag.StringVar(&addr, "addr", "127.0.0.1:8787", "HTTP listen address")
	flag.StringVar(&templateRoot, "templates", ".", "Template root directory for requested template files")
	flag.Parse()

	if handler, err = labelserver.NewHandler(templateRoot); err != nil {
		log.Fatalf("configure server: %v", err)
	}

	mux = http.NewServeMux()
	mux.Handle("/render", handler)

	server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       90 * time.Second,
	}

	log.Printf("panel render server listening on http://%s/render", addr)
	log.Printf("template root: %s", templateRoot)
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failed: %v", err)
	}
}
