package main

import (
	"log"
	"net/http"

	"pokerlab/internal/config"
	apphttp "pokerlab/internal/http"
	"pokerlab/internal/session"
	"pokerlab/internal/sim"
	"pokerlab/internal/table"
	"pokerlab/internal/templates"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	renderer, err := templates.New("web/templates/**/*.gohtml")
	if err != nil {
		log.Fatalf("load templates: %v", err)
	}

	sessions := session.NewManagerWithConfig(session.Config{
		CookieName:   cfg.Session.CookieName,
		MaxTables:    cfg.Session.MaxTables,
		CookieSecure: cfg.Session.CookieSecure,
		CookieMaxAge: cfg.Session.CookieMaxAge,
	})
	engine := sim.NewEngineWithConfig(sim.EngineConfig{
		IntraHandDelay: cfg.Simulation.IntraHandDelay,
		HandPause:      cfg.Simulation.HandPause,
	})
	tables := table.NewManagerWithConfig(engine, sim.RuntimeConfig{
		HistoryLimit:     cfg.Simulation.HistoryLimit,
		SubscriberBuffer: cfg.Simulation.SubscriberBuffer,
	})
	app := apphttp.NewAppWithServicesAndConfig(renderer, sessions, tables, apphttp.Config{
		StreamHeartbeatInterval: cfg.Stream.HeartbeatInterval,
		StreamReplayLimit:       cfg.Stream.ReplayLimit,
		StreamSubscriberBuffer:  cfg.Stream.SubscriberBuffer,
	})

	server := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      app.Routes(),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	log.Printf("listening on http://localhost%s", cfg.HTTP.Addr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server stopped: %v", err)
	}
}
