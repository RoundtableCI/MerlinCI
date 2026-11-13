// Command merlin-worker runs MerlinCI's agent runtime and durable workflows.
// Roles and capabilities run here as configuration; they are not separate
// services (deck slides 5, 8, 12).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/RoundtableCI/MerlinCI/internal/app"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "merlin-worker:", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	env, err := app.LoadEnv()
	if err != nil {
		return err
	}
	ctx := context.Background()
	core, err := app.Open(ctx, env, app.Options{Log: log})
	if err != nil {
		return err
	}
	defer core.Close()

	c, err := app.DialTemporal(env)
	if err != nil {
		return err
	}
	defer c.Close()

	log.Info("worker started", "config", core.Config.Digest, "temporal", env.TemporalAddress)
	return app.RunWorker(c, core)
}
