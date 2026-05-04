package scan

import (
	"bufio"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Yacobolo/toolbelt/gogov/governance/config"
	"github.com/Yacobolo/toolbelt/gogov/governance/model"

	"golang.org/x/tools/go/packages"
)

type fileAccumulator struct {
	model.File
	Symbols []model.Symbol
}

type packageAccumulator struct {
	model.Package
}

type interfaceDef struct {
	FilePath string
	Type     *types.Interface
}

type concreteDef struct {
	FilePath string
	Type     *types.Named
}

func Analyze(ctx context.Context, repo config.Repository, logger *slog.Logger) (model.Snapshot, error) {
	repoRoot, err := filepath.Abs(repo.Root)
	if err != nil {
		return model.Snapshot{}, fmt.Errorf("resolve repo root: %w", err)
	}
	repo.Root = repoRoot

	modulePath, err := readModulePath(repo.Root)
	if err != nil {
		return model.Snapshot{}, err
	}

	loadedPackages, packageErrors, err := loadPackages(ctx, repo.Root)
	if err != nil {
		return model.Snapshot{}, err
	}
	if len(packageErrors) > 0 && logger != nil {
		logger.Warn("package load completed with errors", "count", len(packageErrors), "repo", repo.Root)
	}

	fileAcc, packageAcc, err := discoverFiles(ctx, repo, modulePath)
	if err != nil {
		return model.Snapshot{}, err
	}

	packageDirs := map[string]string{}
	packageFiles := map[string]map[string]struct{}{}
	packageEdges, fileEdges, err := enrichFromPackages(repo, fileAcc, packageAcc, packageDirs, packageFiles, loadedPackages)
	if err != nil {
		return model.Snapshot{}, err
	}

	coverageMap, coverageStatus, coverageErr := collectCoverage(ctx, repo, modulePath, logger)
	applyCoverage(fileAcc, coverageMap)
	applyFanCounts(fileAcc, fileEdges)
	applyPackageEdgeCounts(packageAcc, packageEdges)

	files := make([]model.File, 0, len(fileAcc))
	symbols := make([]model.Symbol, 0)
	for _, item := range fileAcc {
		files = append(files, item.File)
		symbols = append(symbols, item.Symbols...)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].FilePath == symbols[j].FilePath {
			if symbols[i].Line == symbols[j].Line {
				return symbols[i].Name < symbols[j].Name
			}
			return symbols[i].Line < symbols[j].Line
		}
		return symbols[i].FilePath < symbols[j].FilePath
	})

	packagesList := make([]model.Package, 0, len(packageAcc))
	for _, item := range packageAcc {
		packagesList = append(packagesList, item.Package)
	}
	sort.Slice(packagesList, func(i, j int) bool { return packagesList[i].Path < packagesList[j].Path })

	packageEdgeList := make([]model.PackageEdge, 0, len(packageEdges))
	for _, item := range packageEdges {
		packageEdgeList = append(packageEdgeList, item)
	}
	sort.Slice(packageEdgeList, func(i, j int) bool {
		if packageEdgeList[i].FromPath == packageEdgeList[j].FromPath {
			return packageEdgeList[i].ToPath < packageEdgeList[j].ToPath
		}
		return packageEdgeList[i].FromPath < packageEdgeList[j].FromPath
	})

	fileEdgeList := make([]model.FileEdge, 0, len(fileEdges))
	for _, item := range fileEdges {
		fileEdgeList = append(fileEdgeList, item)
	}
	sort.Slice(fileEdgeList, func(i, j int) bool {
		if fileEdgeList[i].FromPath == fileEdgeList[j].FromPath {
			if fileEdgeList[i].ToPath == fileEdgeList[j].ToPath {
				return fileEdgeList[i].Kind < fileEdgeList[j].Kind
			}
			return fileEdgeList[i].ToPath < fileEdgeList[j].ToPath
		}
		return fileEdgeList[i].FromPath < fileEdgeList[j].FromPath
	})

	coverageList := make([]model.CoverageFile, 0, len(coverageMap))
	for _, item := range coverageMap {
		coverageList = append(coverageList, item)
	}
	sort.Slice(coverageList, func(i, j int) bool { return coverageList[i].Path < coverageList[j].Path })

	status := coverageStatus
	if coverageErr != "" && status == model.CoverageStatusFailed {
		logger.Warn("coverage collection failed", "error", coverageErr)
	}

	meta := model.SnapshotMeta{
		RepoRoot:          repo.Root,
		ModulePath:        modulePath,
		CoverageStatus:    status,
		FilesCount:        len(files),
		PackagesCount:     len(packagesList),
		PackageEdgesCount: len(packageEdgeList),
		FileEdgesCount:    len(fileEdgeList),
	}

	return model.Snapshot{
		Meta:         meta,
		Packages:     packagesList,
		Files:        files,
		Symbols:      symbols,
		PackageEdges: packageEdgeList,
		FileEdges:    fileEdgeList,
		Coverage:     coverageList,
	}, nil
}

func readModulePath(repoRoot string) (string, error) {
	file, err := os.Open(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("open go.mod: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	return "", fmt.Errorf("module path not found in go.mod")
}

func discoverFiles(ctx context.Context, repo config.Repository, modulePath string) (map[string]*fileAccumulator, map[string]*packageAccumulator, error) {
	files := map[string]*fileAccumulator{}
	packagesMap := map[string]*packageAccumulator{}
	ignorePaths := config.DefaultIgnorePaths()

	relPaths, err := walkRepositoryGoFiles(repo.Root, ignorePaths)
	if err != nil {
		return nil, nil, err
	}
	ignoredPaths := listIgnoredGitFiles(ctx, repo.Root)
	for _, rel := range relPaths {
		absPath := filepath.Join(repo.Root, filepath.FromSlash(rel))
		if _, err := os.Stat(absPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, fmt.Errorf("stat %s: %w", rel, err)
		}
		src, err := os.ReadFile(absPath)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", rel, err)
		}
		fset := token.NewFileSet()
		fileNode, err := parser.ParseFile(fset, absPath, src, parser.ParseComments)
		if err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", rel, err)
		}

		loc, nonEmpty := countLOC(src)
		dirRel := filepath.ToSlash(filepath.Dir(rel))
		if dirRel == "." {
			dirRel = ""
		}
		packagePath := modulePath
		if dirRel != "" {
			packagePath = modulePath + "/" + dirRel
		}

		item := &fileAccumulator{
			File: model.File{
				Path:        rel,
				Dir:         dirRel,
				PackagePath: packagePath,
				PackageName: fileNode.Name.Name,
				LOC:         loc,
				NonEmptyLOC: nonEmpty,
				IsTest:      strings.HasSuffix(rel, "_test.go"),
				IsGenerated: isGeneratedGoFile(rel, src),
				IsIgnored:   ignoredPaths[rel],
			},
		}
		item.FunctionCount, item.ExportedSymbolCount, item.Symbols = collectFileSymbols(rel, fileNode, fset)
		files[rel] = item

		pkg := packagesMap[packagePath]
		if pkg == nil {
			pkg = &packageAccumulator{
				Package: model.Package{
					Path: packagePath,
					Name: fileNode.Name.Name,
					Dir:  dirRel,
				},
			}
			packagesMap[packagePath] = pkg
		}
		pkg.FileCount++
		if item.IsTest {
			pkg.TestFileCount++
		}
		pkg.LOC += loc
		pkg.NonEmptyLOC += nonEmpty
	}
	return files, packagesMap, nil
}

func listIgnoredGitFiles(ctx context.Context, repoRoot string) map[string]bool {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--others", "--ignored", "--exclude-standard", "-z")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return map[string]bool{}
	}

	ignored := map[string]bool{}
	for _, rel := range strings.Split(string(output), "\x00") {
		rel = strings.TrimSpace(filepath.ToSlash(rel))
		if rel == "" || filepath.Ext(rel) != ".go" {
			continue
		}
		ignored[rel] = true
	}
	return ignored
}

func walkRepositoryGoFiles(repoRoot string, ignorePaths []string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(repoRoot, func(absPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if absPath == repoRoot {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, absPath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if shouldIgnore(rel, ignorePaths) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || filepath.Ext(absPath) != ".go" {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func isGeneratedGoFile(relPath string, src []byte) bool {
	base := strings.ToLower(filepath.Base(relPath))
	if strings.HasSuffix(base, ".gen.go") || strings.HasSuffix(base, ".generated.go") || strings.HasPrefix(base, "zz_generated.") {
		return true
	}

	lines := strings.Split(string(src), "\n")
	limit := 8
	if len(lines) < limit {
		limit = len(lines)
	}
	for i := 0; i < limit; i++ {
		line := strings.ToLower(strings.TrimSpace(lines[i]))
		if strings.Contains(line, "code generated") || strings.Contains(line, "do not edit") {
			return true
		}
	}
	return false
}

func loadPackages(ctx context.Context, repoRoot string) ([]*packages.Package, []error, error) {
	cfg := &packages.Config{
		Context: ctx,
		Dir:     repoRoot,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedTypesSizes,
	}

	items, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, nil, fmt.Errorf("load packages: %w", err)
	}
	allErrs := collectPackageErrors(items)
	if shouldRetryWithDevTags(allErrs) {
		cfg.BuildFlags = []string{"-tags=dev"}
		items, err = packages.Load(cfg, "./...")
		if err != nil {
			return nil, nil, fmt.Errorf("load packages with dev tag: %w", err)
		}
		allErrs = collectPackageErrors(items)
	}
	return items, allErrs, nil
}

func collectPackageErrors(items []*packages.Package) []error {
	allErrs := make([]error, 0)
	for _, item := range items {
		for _, pkgErr := range item.Errors {
			allErrs = append(allErrs, pkgErr)
		}
	}
	return allErrs
}

func shouldRetryWithDevTags(items []error) bool {
	for _, item := range items {
		if strings.Contains(item.Error(), "no matching files found") {
			return true
		}
	}
	return false
}

func enrichFromPackages(
	repo config.Repository,
	fileAcc map[string]*fileAccumulator,
	packageAcc map[string]*packageAccumulator,
	packageDirs map[string]string,
	packageFiles map[string]map[string]struct{},
	loadedPackages []*packages.Package,
) (map[string]model.PackageEdge, map[string]model.FileEdge, error) {
	packageEdges := map[string]model.PackageEdge{}
	fileEdges := map[string]model.FileEdge{}
	ignorePaths := config.DefaultIgnorePaths()

	var interfaces []interfaceDef
	var concretes []concreteDef

	for _, pkg := range loadedPackages {
		pkgPath := pkg.PkgPath
		for _, absFile := range pkg.CompiledGoFiles {
			rel, ok := relativeRepoPath(repo.Root, absFile)
			if !ok || shouldIgnore(rel, ignorePaths) {
				continue
			}
			if _, found := fileAcc[rel]; !found {
				continue
			}
			packageFiles[pkgPath] = ensureSet(packageFiles[pkgPath])
			packageFiles[pkgPath][rel] = struct{}{}

			if relDir := filepath.ToSlash(filepath.Dir(rel)); relDir != "." {
				packageDirs[pkgPath] = relDir
			} else if _, exists := packageDirs[pkgPath]; !exists {
				packageDirs[pkgPath] = ""
			}

			if pkgInfo := packageAcc[pkgPath]; pkgInfo != nil {
				pkgInfo.Name = pkg.Name
				pkgInfo.Dir = packageDirs[pkgPath]
			}
			if file := fileAcc[rel]; file != nil {
				file.PackagePath = pkgPath
				file.PackageName = pkg.Name
			}
		}

		for importedPath, importedPkg := range pkg.Imports {
			if importedPkg == nil {
				continue
			}
			sourceDir := packageDirs[pkgPath]
			targetDir := packageDirs[importedPath]
			if sourceDir == "" && pkgPath != "" {
				sourceDir = inferDirFromPackagePath(pkgPath, packageAcc)
			}
			if targetDir == "" && importedPath != "" {
				targetDir = inferDirFromPackagePath(importedPath, packageAcc)
			}
			if shouldIgnore(sourceDir, ignorePaths) || shouldIgnore(targetDir, ignorePaths) {
				continue
			}
			if _, ok := packageAcc[importedPath]; !ok {
				continue
			}
			key := pkgPath + "->" + importedPath
			edge := packageEdges[key]
			edge.FromPath = pkgPath
			edge.ToPath = importedPath
			edge.Weight++
			packageEdges[key] = edge
		}

		for fileIndex, syntax := range pkg.Syntax {
			if fileIndex >= len(pkg.CompiledGoFiles) {
				continue
			}
			rel, ok := relativeRepoPath(repo.Root, pkg.CompiledGoFiles[fileIndex])
			if !ok || shouldIgnore(rel, ignorePaths) {
				continue
			}
			if _, found := fileAcc[rel]; !found {
				continue
			}

			if pkg.TypesInfo != nil && pkg.Fset != nil {
				for ident, obj := range pkg.TypesInfo.Uses {
					if ident == nil || obj == nil {
						continue
					}
					useFile := positionFile(pkg.Fset, ident.Pos())
					defFile := positionFile(pkg.Fset, obj.Pos())
					if useFile == "" || defFile == "" || useFile == defFile {
						continue
					}
					useRel, okUse := relativeRepoPath(repo.Root, useFile)
					defRel, okDef := relativeRepoPath(repo.Root, defFile)
					if !okUse || !okDef || useRel == defRel {
						continue
					}
					if shouldIgnore(useRel, ignorePaths) || shouldIgnore(defRel, ignorePaths) {
						continue
					}
					if _, ok := fileAcc[useRel]; !ok {
						continue
					}
					if _, ok := fileAcc[defRel]; !ok {
						continue
					}
					addFileEdge(fileEdges, useRel, defRel, "symbol")
				}
			}

			if pkg.TypesInfo == nil || pkg.Fset == nil {
				continue
			}
			ast.Inspect(syntax, func(node ast.Node) bool {
				typeSpec, ok := node.(*ast.TypeSpec)
				if !ok {
					return true
				}
				obj := pkg.TypesInfo.Defs[typeSpec.Name]
				if obj == nil {
					return true
				}
				typeName, ok := obj.(*types.TypeName)
				if !ok {
					return true
				}
				named, ok := typeName.Type().(*types.Named)
				if !ok {
					return true
				}
				defFile := positionFile(pkg.Fset, obj.Pos())
				relFile, ok := relativeRepoPath(repo.Root, defFile)
				if !ok || shouldIgnore(relFile, ignorePaths) {
					return true
				}
				if iface, ok := named.Underlying().(*types.Interface); ok {
					interfaces = append(interfaces, interfaceDef{FilePath: relFile, Type: iface.Complete()})
					return true
				}
				concretes = append(concretes, concreteDef{FilePath: relFile, Type: named})
				return true
			})
		}
	}

	for _, concrete := range concretes {
		for _, iface := range interfaces {
			if concrete.FilePath == iface.FilePath {
				continue
			}
			if types.Implements(concrete.Type, iface.Type) || types.Implements(types.NewPointer(concrete.Type), iface.Type) {
				addFileEdge(fileEdges, concrete.FilePath, iface.FilePath, "implements")
			}
		}
	}

	return packageEdges, fileEdges, nil
}

func collectCoverage(ctx context.Context, repo config.Repository, modulePath string, logger *slog.Logger) (map[string]model.CoverageFile, string, string) {
	outputPath := repo.CoverageOutputPath
	if err := os.MkdirAll(filepath.Dir(outputPath), 0750); err != nil {
		return map[string]model.CoverageFile{}, model.CoverageStatusFailed, err.Error()
	}
	_ = os.Remove(outputPath)

	out, err := runCoverageCommand(ctx, repo.Root, config.DefaultCoverageArgs(outputPath))
	items, parseErr := parseCoverageFile(outputPath, repo.Root, modulePath)
	if parseErr != nil {
		if err != nil && logger != nil {
			logger.Warn("coverage command failed", "error", err, "output", string(out))
		}
		return map[string]model.CoverageFile{}, model.CoverageStatusFailed, parseErr.Error()
	}
	if err != nil {
		if len(items) == 0 {
			devOut, devErr := runCoverageCommand(ctx, repo.Root, config.DefaultCoverageArgsWithDevTag(outputPath))
			devItems, devParseErr := parseCoverageFile(outputPath, repo.Root, modulePath)
			if devParseErr == nil && len(devItems) > 0 {
				if logger != nil {
					logger.Warn("coverage command succeeded with dev build tag fallback", "error", devErr, "output", string(devOut))
				}
				return devItems, model.CoverageStatusAvailable, strings.TrimSpace(string(devOut))
			}
		}
		if len(items) > 0 {
			if logger != nil {
				logger.Warn("coverage command returned a non-zero exit code but produced a usable profile", "error", err, "output", string(out))
			}
			return items, model.CoverageStatusAvailable, strings.TrimSpace(string(out))
		}
		if logger != nil {
			logger.Warn("coverage command failed", "error", err, "output", string(out))
		}
		return map[string]model.CoverageFile{}, model.CoverageStatusFailed, strings.TrimSpace(string(out))
	}
	if len(items) == 0 {
		return map[string]model.CoverageFile{}, model.CoverageStatusMissing, ""
	}
	return items, model.CoverageStatusAvailable, ""
}

func runCoverageCommand(ctx context.Context, repoRoot string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = repoRoot
	return cmd.CombinedOutput()
}

func parseCoverageFile(profilePath string, repoRoot string, modulePath string) (map[string]model.CoverageFile, error) {
	file, err := os.Open(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]model.CoverageFile{}, nil
		}
		return nil, fmt.Errorf("open coverage profile: %w", err)
	}
	defer file.Close()

	items := map[string]model.CoverageFile{}
	scanner := bufio.NewScanner(file)
	firstLine := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if firstLine {
			firstLine = false
			if strings.HasPrefix(line, "mode:") {
				continue
			}
		}
		parts := strings.Fields(line)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid coverage line: %s", line)
		}
		filePart, statementsPart, countPart := parts[0], parts[1], parts[2]
		filePath := normalizeCoveragePath(strings.Split(filePart, ":")[0], repoRoot, modulePath)
		if filePath == "" {
			continue
		}
		statements, err := strconv.Atoi(statementsPart)
		if err != nil {
			return nil, fmt.Errorf("parse statements in %s: %w", line, err)
		}
		count, err := strconv.Atoi(countPart)
		if err != nil {
			return nil, fmt.Errorf("parse count in %s: %w", line, err)
		}
		item := items[filePath]
		item.Path = filePath
		item.TotalStatements += statements
		if count > 0 {
			item.CoveredStatements += statements
		}
		items[filePath] = item
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan coverage profile: %w", err)
	}
	for key, item := range items {
		if item.TotalStatements > 0 {
			item.CoveragePct = float64(item.CoveredStatements) * 100 / float64(item.TotalStatements)
		}
		items[key] = item
	}
	return items, nil
}

func normalizeCoveragePath(value string, repoRoot string, modulePath string) string {
	value = filepath.ToSlash(value)
	if filepath.IsAbs(value) {
		rel, err := filepath.Rel(repoRoot, value)
		if err == nil {
			return filepath.ToSlash(rel)
		}
	}
	if strings.HasPrefix(value, modulePath+"/") {
		return strings.TrimPrefix(value, modulePath+"/")
	}
	if strings.HasPrefix(value, repoRoot+"/") {
		return strings.TrimPrefix(value, repoRoot+"/")
	}
	if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(value))); err == nil {
		return value
	}
	return ""
}

func applyCoverage(files map[string]*fileAccumulator, coverage map[string]model.CoverageFile) {
	for path, item := range coverage {
		file := files[path]
		if file == nil {
			continue
		}
		file.CoveredStatements = item.CoveredStatements
		file.TotalStatements = item.TotalStatements
		coveragePct := item.CoveragePct
		file.CoveragePct = &coveragePct
	}
}

func applyFanCounts(files map[string]*fileAccumulator, edges map[string]model.FileEdge) {
	for _, edge := range edges {
		if source := files[edge.FromPath]; source != nil {
			source.FanOut++
		}
		if target := files[edge.ToPath]; target != nil {
			target.FanIn++
		}
	}
}

func applyPackageEdgeCounts(packagesMap map[string]*packageAccumulator, edges map[string]model.PackageEdge) {
	for _, edge := range edges {
		if source := packagesMap[edge.FromPath]; source != nil {
			source.ImportsCount++
		}
		if target := packagesMap[edge.ToPath]; target != nil {
			target.ImportedByCount++
		}
	}
}

func collectFileSymbols(relPath string, file *ast.File, fset *token.FileSet) (int, int, []model.Symbol) {
	functionCount := 0
	exportedCount := 0
	symbols := make([]model.Symbol, 0)

	for _, decl := range file.Decls {
		switch item := decl.(type) {
		case *ast.FuncDecl:
			functionCount++
			name := item.Name.Name
			exported := ast.IsExported(name)
			if exported {
				exportedCount++
			}
			kind := "func"
			if item.Recv != nil {
				kind = "method"
			}
			symbols = append(symbols, model.Symbol{
				FilePath: relPath,
				Name:     name,
				Kind:     kind,
				Line:     fset.Position(item.Pos()).Line,
				Exported: exported,
			})
		case *ast.GenDecl:
			for _, spec := range item.Specs {
				switch typed := spec.(type) {
				case *ast.TypeSpec:
					exported := ast.IsExported(typed.Name.Name)
					if exported {
						exportedCount++
					}
					symbols = append(symbols, model.Symbol{
						FilePath: relPath,
						Name:     typed.Name.Name,
						Kind:     "type",
						Line:     fset.Position(typed.Pos()).Line,
						Exported: exported,
					})
				case *ast.ValueSpec:
					for _, name := range typed.Names {
						exported := ast.IsExported(name.Name)
						if exported {
							exportedCount++
						}
						kind := strings.ToLower(item.Tok.String())
						symbols = append(symbols, model.Symbol{
							FilePath: relPath,
							Name:     name.Name,
							Kind:     kind,
							Line:     fset.Position(name.Pos()).Line,
							Exported: exported,
						})
					}
				}
			}
		}
	}

	return functionCount, exportedCount, symbols
}

func countLOC(src []byte) (int, int) {
	lines := strings.Split(string(src), "\n")
	loc := 0
	nonEmpty := 0
	for _, line := range lines {
		loc++
		if strings.TrimSpace(line) != "" {
			nonEmpty++
		}
	}
	return loc, nonEmpty
}

func shouldIgnore(rel string, patterns []string) bool {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	rel = strings.TrimPrefix(rel, "/")
	for _, rawPattern := range patterns {
		pattern := strings.TrimSpace(filepath.ToSlash(rawPattern))
		pattern = strings.TrimPrefix(pattern, "./")
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				return true
			}
		}
		if rel == pattern || strings.HasPrefix(rel, pattern+"/") {
			return true
		}
		if matched, _ := path.Match(pattern, rel); matched {
			return true
		}
	}
	return false
}

func addFileEdge(edges map[string]model.FileEdge, from string, to string, kind string) {
	key := from + "->" + to + "::" + kind
	item := edges[key]
	item.FromPath = from
	item.ToPath = to
	item.Kind = kind
	item.Weight++
	edges[key] = item
}

func ensureSet(current map[string]struct{}) map[string]struct{} {
	if current == nil {
		return map[string]struct{}{}
	}
	return current
}

func relativeRepoPath(repoRoot string, absPath string) (string, bool) {
	rel, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return "", false
	}
	if strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func positionFile(fset *token.FileSet, pos token.Pos) string {
	if !pos.IsValid() {
		return ""
	}
	return fset.PositionFor(pos, false).Filename
}

func inferDirFromPackagePath(pkgPath string, packagesMap map[string]*packageAccumulator) string {
	if pkg := packagesMap[pkgPath]; pkg != nil {
		return pkg.Dir
	}
	return ""
}
