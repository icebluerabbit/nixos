// Command mousetrap is the MyAnonaMouse grab authority. See README.md for the
// requirements. main is thin: load config, wire dependencies, serve.
package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"mousetrap/app"
	"mousetrap/config"
	"mousetrap/server"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()
	a, err := app.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()
	slog.Info("mousetrap listening", "addr", cfg.Addr())
	if err := http.ListenAndServe(cfg.Addr(), server.New(a)); err != nil {
		log.Fatal(err)
	}
}
