package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Yacobolo/toolbelt/gogov/governance"
	"github.com/Yacobolo/toolbelt/gogov/governance/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "gogov: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: gogov <serve|refresh>")
	}

	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	app, err := governance.New(cfg, logger)
	if err != nil {
		return err
	}
	defer func() { _ = app.Close() }()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch os.Args[1] {
	case "serve":
		return app.Serve(ctx)
	case "refresh":
		repoID, err := parseRefreshRepo(cfg, os.Args[2:])
		if err != nil {
			return err
		}
		run, err := app.Refresh(ctx, repoID)
		if err != nil {
			if errors.Is(err, governance.ErrRefreshRunning) {
				return fmt.Errorf("refresh already running")
			}
			return err
		}
		fmt.Printf("repo=%s run=%s status=%s files=%d packages=%d coverage=%s\n",
			repoID,
			run.ID,
			run.Status,
			run.FilesCount,
			run.PackagesCount,
			run.CoverageStatus,
		)
		return nil
	default:
		return fmt.Errorf("unknown command %q", os.Args[1])
	}
}

func parseRefreshRepo(cfg config.Config, args []string) (string, error) {
	fs := flag.NewFlagSet("refresh", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoID := fs.String("repo", "", "repository id")
	if err := fs.Parse(args); err != nil {
		return "", err
	}

	if strings.TrimSpace(*repoID) != "" {
		return strings.TrimSpace(*repoID), nil
	}
	if len(cfg.Repositories) == 1 {
		return cfg.Repositories[0].ID, nil
	}

	valid := make([]string, 0, len(cfg.Repositories))
	for _, repo := range cfg.Repositories {
		valid = append(valid, repo.ID)
	}
	return "", fmt.Errorf("refresh requires --repo when multiple repositories are configured (valid: %s)", strings.Join(valid, ", "))
}
