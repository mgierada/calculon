// Package server serves the dashboards over SSH: `ssh calculon` opens the same
// fullscreen app the local binary runs, showing the portfolio of whichever user
// the client's public key belongs to.
package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/ssh"
	"charm.land/wish/v2"
	"charm.land/wish/v2/activeterm"
	"charm.land/wish/v2/bubbletea"
	"charm.land/wish/v2/logging"

	"github.com/mgierada/calculon/internal/auth"
	"github.com/mgierada/calculon/internal/db"
	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/ui"
)

// shutdownTimeout bounds how long open sessions get to finish on shutdown.
const shutdownTimeout = 10 * time.Second

// Options configure the SSH server.
type Options struct {
	Addr        string
	HostKeyPath string
	Conn        *sql.DB
	Dashboards  []ui.Dashboard
	Report      portfolio.Options
}

// Run serves until ctx is cancelled, then shuts down gracefully.
func Run(ctx context.Context, opts Options) error {
	if err := os.MkdirAll(filepath.Dir(opts.HostKeyPath), 0o700); err != nil {
		return fmt.Errorf("failed to create host key directory: %w", err)
	}

	srv, err := wish.NewServer(
		wish.WithAddress(opts.Addr),
		wish.WithHostKeyPath(opts.HostKeyPath),
		wish.WithPublicKeyAuth(func(_ ssh.Context, key ssh.PublicKey) bool {
			return knownKey(opts.Conn, key)
		}),
		wish.WithMiddleware(
			bubbletea.Middleware(handler(opts)),
			activeterm.Middleware(),
			logging.Middleware(),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create ssh server: %w", err)
	}

	errs := make(chan error, 1)
	go func() {
		log.Printf("serving calculon over ssh on %s", opts.Addr)
		errs <- srv.ListenAndServe()
	}()

	select {
	case err := <-errs:
		if errors.Is(err, ssh.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		return fmt.Errorf("failed to shut down: %w", err)
	}
	return nil
}

// knownKey admits keys registered to some user. It only answers yes or no:
// the callback can run for keys the client never proves it holds, so the
// session looks the user up again from the key it actually authenticated with.
func knownKey(conn *sql.DB, key ssh.PublicKey) bool {
	_, err := db.UserByKey(conn, auth.Fingerprint(key))
	return err == nil
}

// handler builds one app per session, scoped to the session's user.
func handler(opts Options) bubbletea.Handler {
	return func(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
		key := sess.PublicKey()
		if key == nil {
			wish.Fatalln(sess, "public key authentication required")
			return nil, nil
		}
		user, err := db.UserByKey(opts.Conn, auth.Fingerprint(key))
		if err != nil {
			wish.Fatalln(sess, "unknown key")
			return nil, nil
		}

		load := func(scope *model.AccountKey) (portfolio.Report, error) {
			return portfolio.Load(opts.Conn, user, opts.Report, scope)
		}
		return ui.NewApp(opts.Dashboards, load), nil
	}
}
