package web

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/evanw/esbuild/pkg/api"
)

const (
	assetsRoutePrefix = "/assets/"
	assetsScriptName  = "governance-app.js"
	assetsStyleName   = "governance-app.css"
)

func (s *Server) EnsureAssetsBuilt(_ context.Context) error {
	nodeModulesPath := filepath.Join(s.cfg.HostRoot, "web", "node_modules")
	if _, err := os.Stat(nodeModulesPath); err != nil {
		if os.IsNotExist(err) {
			if assetsExist(s.assetsDir()) {
				return nil
			}
			return fmt.Errorf("frontend dependencies are missing; run `pnpm --dir web install`")
		}
		return fmt.Errorf("stat frontend dependencies: %w", err)
	}

	outdir := s.assetsDir()
	if err := os.MkdirAll(outdir, 0750); err != nil {
		return fmt.Errorf("create governance asset directory: %w", err)
	}

	result := api.Build(api.BuildOptions{
		EntryPointsAdvanced: []api.EntryPoint{
			{
				InputPath:  filepath.Join(s.cfg.HostRoot, "governance", "frontend", "index.ts"),
				OutputPath: "governance-app",
			},
		},
		Bundle:            true,
		Format:            api.FormatESModule,
		LogLevel:          api.LogLevelSilent,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		MinifyWhitespace:  true,
		NodePaths:         []string{nodeModulesPath},
		Outdir:            outdir,
		Sourcemap:         api.SourceMapLinked,
		Target:            api.ES2020,
		Write:             true,
	})

	if len(result.Errors) > 0 {
		errs := make([]error, 0, len(result.Errors))
		for _, item := range result.Errors {
			errs = append(errs, errors.New(item.Text))
		}
		return fmt.Errorf("build governance frontend: %w", errors.Join(errs...))
	}
	return nil
}

func (s *Server) assetsDir() string {
	return filepath.Join(s.cfg.HostRoot, "governance", "web", "static")
}

func assetsExist(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, assetsScriptName)); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, assetsStyleName)); err != nil {
		return false
	}
	return true
}
