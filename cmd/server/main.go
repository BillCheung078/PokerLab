package main

import (
	"log"
	"net/http"
	"os"

	apphttp "pokerlab/internal/http"
	"pokerlab/internal/templates"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	renderer, err := templates.New("web/templates/**/*.gohtml")
	if err != nil {
		log.Fatalf("load templates: %v", err)
	}

	app := apphttp.NewApp(renderer)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: app.Routes(),
	}

	log.Printf("listening on http://localhost:%s", port)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server stopped: %v", err)
	}
}
