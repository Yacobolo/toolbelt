package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"

	cligoemit "github.com/Yacobolo/toolbelt/apigen/emit/cligo"
	requestmodelgoemit "github.com/Yacobolo/toolbelt/apigen/emit/requestmodelgo"
	servergoemit "github.com/Yacobolo/toolbelt/apigen/emit/servergo"
	"github.com/Yacobolo/toolbelt/apigen/ir"
)

type generatedOutputChange struct {
	Path    string
	Content []byte
	Remove  bool
}

func generatePartitionedServer(doc ir.Document, plan goPackagePlan) error {
	changes, err := renderPartitionedServerDocument(doc, plan)
	if err != nil {
		return err
	}
	if err := applyGeneratedOutputChanges(changes); err != nil {
		return fmt.Errorf("apply generated outputs: %w", err)
	}
	return nil
}

func generatePartitionedAll(doc ir.Document, plan goPackagePlan, config commandConfig) error {
	changes, err := renderPartitionedServerDocument(doc, plan)
	if err != nil {
		return err
	}
	if config.GenerateCLI {
		cli, err := cligoemit.Emit(doc, cligoemit.Options{PackageName: config.CLIPackage})
		if err != nil {
			return fmt.Errorf("emit global cli: %w", err)
		}
		formattedCLI, err := format.Source(cli)
		if err != nil {
			return fmt.Errorf("format global cli: %w", err)
		}
		changes = append(changes, generatedOutputChange{Path: config.CLIOut, Content: formattedCLI})
	}
	if err := applyGeneratedOutputChanges(changes); err != nil {
		return fmt.Errorf("apply generated outputs: %w", err)
	}
	return nil
}

func renderPartitionedServerDocument(doc ir.Document, plan goPackagePlan) ([]generatedOutputChange, error) {
	partitions, err := planGoPackagePartitions(doc, plan)
	if err != nil {
		return nil, fmt.Errorf("plan packages: %w", err)
	}
	projections, err := projectGoPackagePartitions(doc, partitions)
	if err != nil {
		return nil, fmt.Errorf("project packages: %w", err)
	}
	changes, err := renderPartitionedServer(projections)
	if err != nil {
		return nil, err
	}
	return changes, nil
}

func renderPartitionedServer(projections []goPackageProjection) ([]generatedOutputChange, error) {
	changes := make([]generatedOutputChange, 0, len(projections)*2)
	for _, projection := range projections {
		output := projection.Partition.Output
		imports, err := partitionContractImports(projection)
		if err != nil {
			return nil, fmt.Errorf("resolve imports for %s: %w", output.ImportPath, err)
		}
		models, err := requestmodelgoemit.Emit(projection.Document, requestmodelgoemit.Options{
			PackageName:     output.Package,
			ContractImports: emitterContractImports(imports),
		})
		if err != nil {
			return nil, fmt.Errorf("emit request models for %s: %w", output.ImportPath, err)
		}
		formattedModels, err := format.Source(models)
		if err != nil {
			return nil, fmt.Errorf("format request models for %s: %w", output.ImportPath, err)
		}
		changes = append(changes, generatedOutputChange{
			Path:    filepath.Join(output.Dir, output.RequestModelsFile),
			Content: formattedModels,
		})

		serverPath := filepath.Join(output.Dir, output.ServerFile)
		if len(projection.Document.Endpoints) == 0 {
			changes = append(changes, generatedOutputChange{Path: serverPath, Remove: true})
			continue
		}
		if err := servergoemit.ValidateOperationIDs(projection.Document); err != nil {
			return nil, fmt.Errorf("validate operation ids for %s: %w", output.ImportPath, err)
		}
		server, err := servergoemit.Emit(projection.Document, servergoemit.Options{
			PackageName: output.Package,
		})
		if err != nil {
			return nil, fmt.Errorf("emit server for %s: %w", output.ImportPath, err)
		}
		formattedServer, err := format.Source(server)
		if err != nil {
			return nil, fmt.Errorf("format server for %s: %w", output.ImportPath, err)
		}
		changes = append(changes, generatedOutputChange{
			Path:    serverPath,
			Content: formattedServer,
		})
	}
	return changes, nil
}

func applyGeneratedOutputChanges(changes []generatedOutputChange) error {
	ordered := append([]generatedOutputChange(nil), changes...)
	sort.Slice(ordered, func(left, right int) bool {
		return ordered[left].Path < ordered[right].Path
	})
	seen := map[string]struct{}{}
	staged := make(map[string]string, len(ordered))
	cleanup := func() {
		for _, path := range staged {
			_ = os.Remove(path)
		}
	}
	defer cleanup()

	for _, change := range ordered {
		path := filepath.Clean(change.Path)
		if path == "." || path == "" {
			return fmt.Errorf("generated output path is required")
		}
		if _, exists := seen[path]; exists {
			return fmt.Errorf("generated output path %s is declared more than once", path)
		}
		seen[path] = struct{}{}
		if change.Remove && len(change.Content) > 0 {
			return fmt.Errorf("generated output %s cannot be written and removed", path)
		}
	}

	for _, change := range ordered {
		path := filepath.Clean(change.Path)
		if change.Remove {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("create output directory for %s: %w", path, err)
		}
		file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
		if err != nil {
			return fmt.Errorf("stage output %s: %w", path, err)
		}
		tempPath := file.Name()
		content := normalizedGeneratedContent(change.Content)
		if _, err := file.Write(content); err != nil {
			_ = file.Close()
			_ = os.Remove(tempPath)
			return fmt.Errorf("stage output %s: %w", path, err)
		}
		if err := file.Chmod(0o600); err != nil {
			_ = file.Close()
			_ = os.Remove(tempPath)
			return fmt.Errorf("set staged output permissions for %s: %w", path, err)
		}
		if err := file.Close(); err != nil {
			_ = os.Remove(tempPath)
			return fmt.Errorf("close staged output %s: %w", path, err)
		}
		staged[path] = tempPath
	}

	for _, change := range ordered {
		path := filepath.Clean(change.Path)
		if change.Remove {
			continue
		}
		tempPath := staged[path]
		if err := os.Rename(tempPath, path); err != nil {
			return fmt.Errorf("replace generated output %s: %w", path, err)
		}
		delete(staged, path)
	}
	for _, change := range ordered {
		if !change.Remove {
			continue
		}
		path := filepath.Clean(change.Path)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale generated output %s: %w", path, err)
		}
	}
	return nil
}

func normalizedGeneratedContent(content []byte) []byte {
	normalized := bytes.TrimSpace(content)
	return append(normalized, '\n')
}
