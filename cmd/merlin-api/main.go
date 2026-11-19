// Command merlin-api runs the MerlinCI control plane: the API a person uses
// to start work, inspect runs and artifacts, approve or refuse actions, and
// pause MerlinCI (deck slides 6, 10, 11, 15).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RoundtableCI/MerlinCI/internal/api"
	"github.com/RoundtableCI/MerlinCI/internal/app"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "merlin-api:", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	env, err := app.LoadEnv()
	if err != nil {
		return err
	}
	// The API can start work and approve actions, so it never runs open.
	if len(env.APIToken) < api.MinTokenLength {
		return fmt.Errorf("MERLIN_API_TOKEN must be set to a secret of at least %d characters", api.MinTokenLength)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	server := &api.Server{
		Config:    core.Config,
		Store:     core.Store,
		Ledger:    core.Ledger,
		Blobs:     core.Blobs,
		Workflows: app.Workflows{Client: c, TaskQueue: env.TaskQueue},
		Token:     env.APIToken,
		Log:       log,
	}
	httpServer := &http.Server{
		Addr:              env.APIAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	errc := make(chan error, 1)
	go func() { errc <- httpServer.ListenAndServe() }()
	log.Info("api started", "addr", env.APIAddr, "config", core.Config.Digest)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
