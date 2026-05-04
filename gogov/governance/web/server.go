package web

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Yacobolo/toolbelt/gogov/governance/config"
	"github.com/Yacobolo/toolbelt/gogov/governance/model"
	"github.com/Yacobolo/toolbelt/gogov/governance/store"
	"github.com/Yacobolo/toolbelt/gogov/internal/render"

	"github.com/go-chi/chi/v5"
	g "maragu.dev/gomponents"
)

type repoRuntime interface {
	Store(repoID string) (*store.Store, bool)
	StartRefresh(ctx context.Context, repoID string) (string, error)
}

type Server struct {
	cfg     config.Config
	logger  *slog.Logger
	runtime repoRuntime
}

func New(cfg config.Config, logger *slog.Logger, runtime repoRuntime) *Server {
	return &Server{
		cfg:     cfg,
		logger:  logger,
		runtime: runtime,
	}
}

func (s *Server) Routes() http.Handler {
	router := chi.NewRouter()
	router.Get("/favicon.ico", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	router.Handle(assetsRoutePrefix+"*", http.StripPrefix(assetsRoutePrefix, http.FileServer(http.Dir(s.assetsDir()))))
	router.Get("/", s.home)
	router.Route("/repos/{repoID}", func(r chi.Router) {
		r.Post("/refresh", s.refresh)
		r.Get("/", s.dashboard)
		r.Get("/runs", s.runs)
		r.Get("/files", s.files)
		r.Get("/files/*", s.fileDetail)
		r.Get("/packages", s.packages)
		r.Get("/packages/*", s.packageDetail)
	})
	return router
}

type repoSummary struct {
	Repo    config.Repository
	Meta    *model.SnapshotMeta
	LastRun *model.Run
}

type layoutData struct {
	Title        string
	Message      string
	Section      string
	Breadcrumbs  []breadcrumbItem
	StatusLabel  string
	StatusTone   string
	Repositories []config.Repository
	ActiveRepo   *config.Repository
	RefreshPath  string
	MainClass    string
	ContentClass string
}

type homeData struct {
	Summaries []repoSummary
}

type dashboardData struct {
	RepoID      string
	Meta        *model.SnapshotMeta
	Runs        []model.Run
	BigFiles    []model.File
	HotPackages []model.Package
}

type runsData struct {
	RepoID string
	Meta   *model.SnapshotMeta
	Runs   []model.Run
}

type filesData struct {
	RepoID        string
	Meta          *model.SnapshotMeta
	Files         []model.File
	Filter        string
	PackageFilter string
	Sort          string
}

type fileDetailData struct {
	RepoID       string
	Meta         *model.SnapshotMeta
	File         model.File
	ActiveTab    string
	Symbols      []model.Symbol
	Inbound      []model.FileEdge
	Outbound     []model.FileEdge
	RelatedTests []model.File
	Graph        graphResponse
	Source       fileSourceResponse
}

type packagesData struct {
	RepoID   string
	Meta     *model.SnapshotMeta
	Packages []model.Package
	Graph    graphResponse
}

type packageDetailData struct {
	RepoID    string
	Meta      *model.SnapshotMeta
	Package   model.Package
	ActiveTab string
	Files     []model.File
	Inbound   []model.PackageEdge
	Outbound  []model.PackageEdge
	Graph     graphResponse
}

type repoContext struct {
	Repo  config.Repository
	Store *store.Store
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	summaries := make([]repoSummary, 0, len(s.cfg.Repositories))
	for _, repo := range s.cfg.Repositories {
		st, ok := s.runtime.Store(repo.ID)
		if !ok {
			continue
		}
		meta, _ := snapshotMeta(r.Context(), st)
		lastRun, _ := latestRun(r.Context(), st)
		summaries = append(summaries, repoSummary{
			Repo:    repo,
			Meta:    meta,
			LastRun: lastRun,
		})
	}

	s.render(w, governancePage(layoutData{
		Title:        "Repository Catalog",
		Message:      r.URL.Query().Get("message"),
		Section:      "home",
		Breadcrumbs:  []breadcrumbItem{{Label: "Catalog"}},
		Repositories: s.cfg.Repositories,
	}, homeView(homeData{Summaries: summaries})))
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	rc, err := s.resolveRepo(r)
	if err != nil {
		s.writeRepoError(w, r, err)
		return
	}

	meta, _ := snapshotMeta(r.Context(), rc.Store)
	runs, err := rc.Store.ListRuns(r.Context(), 12)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	files, err := rc.Store.ListFiles(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].LOC == files[j].LOC {
			return files[i].Path < files[j].Path
		}
		return files[i].LOC > files[j].LOC
	})
	if len(files) > 10 {
		files = files[:10]
	}

	packagesList, err := rc.Store.ListPackages(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Slice(packagesList, func(i, j int) bool {
		if packagesList[i].LOC == packagesList[j].LOC {
			return packagesList[i].Path < packagesList[j].Path
		}
		return packagesList[i].LOC > packagesList[j].LOC
	})
	if len(packagesList) > 10 {
		packagesList = packagesList[:10]
	}

	s.render(w, governancePage(s.repoLayout(
		rc.Repo,
		"overview",
		"Repository Overview",
		r.URL.Query().Get("message"),
		[]breadcrumbItem{{Label: "Catalog", Href: "/"}, {Label: rc.Repo.Name}},
		snapshotStatusLabel(meta),
		snapshotStatusTone(meta),
	), dashboardView(dashboardData{
		RepoID:      rc.Repo.ID,
		Meta:        meta,
		Runs:        runs,
		BigFiles:    files,
		HotPackages: packagesList,
	})))
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	rc, err := s.resolveRepo(r)
	if err != nil {
		s.writeRepoError(w, r, err)
		return
	}

	runID, err := s.runtime.StartRefresh(r.Context(), rc.Repo.ID)
	if err != nil {
		http.Redirect(w, r, repoRunsHref(rc.Repo.ID)+"?message="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, repoRunsHref(rc.Repo.ID)+"?message="+url.QueryEscape("Refresh started: "+runID), http.StatusSeeOther)
}

func (s *Server) runs(w http.ResponseWriter, r *http.Request) {
	rc, err := s.resolveRepo(r)
	if err != nil {
		s.writeRepoError(w, r, err)
		return
	}

	meta, _ := snapshotMeta(r.Context(), rc.Store)
	runs, err := rc.Store.ListRuns(r.Context(), 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, governancePage(s.repoLayout(
		rc.Repo,
		"runs",
		"Scan History",
		r.URL.Query().Get("message"),
		[]breadcrumbItem{{Label: "Catalog", Href: "/"}, {Label: rc.Repo.Name, Href: repoBaseHref(rc.Repo.ID)}, {Label: "Runs"}},
		strconv.Itoa(len(runs))+" runs",
		"neutral",
	), runsView(runsData{
		RepoID: rc.Repo.ID,
		Meta:   meta,
		Runs:   runs,
	})))
}

func (s *Server) files(w http.ResponseWriter, r *http.Request) {
	rc, err := s.resolveRepo(r)
	if err != nil {
		s.writeRepoError(w, r, err)
		return
	}

	meta, _ := snapshotMeta(r.Context(), rc.Store)
	files, err := rc.Store.ListFiles(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filter := strings.TrimSpace(r.URL.Query().Get("q"))
	packageFilter := strings.TrimSpace(r.URL.Query().Get("package"))
	sortBy := strings.TrimSpace(r.URL.Query().Get("sort"))
	if sortBy == "" {
		sortBy = "loc"
	}

	filtered := make([]model.File, 0, len(files))
	for _, item := range files {
		if filter != "" && !strings.Contains(strings.ToLower(item.Path), strings.ToLower(filter)) {
			continue
		}
		if packageFilter != "" && item.PackagePath != packageFilter {
			continue
		}
		filtered = append(filtered, item)
	}
	sortFiles(filtered, sortBy)

	s.render(w, governancePage(s.repoLayout(
		rc.Repo,
		"files",
		"File Catalog",
		"",
		[]breadcrumbItem{{Label: "Catalog", Href: "/"}, {Label: rc.Repo.Name, Href: repoBaseHref(rc.Repo.ID)}, {Label: "Files"}},
		strconv.Itoa(len(filtered))+" files",
		"neutral",
	), filesView(filesData{
		RepoID:        rc.Repo.ID,
		Meta:          meta,
		Files:         filtered,
		Filter:        filter,
		PackageFilter: packageFilter,
		Sort:          sortBy,
	})))
}

func (s *Server) fileDetail(w http.ResponseWriter, r *http.Request) {
	rc, err := s.resolveRepo(r)
	if err != nil {
		s.writeRepoError(w, r, err)
		return
	}

	meta, _ := snapshotMeta(r.Context(), rc.Store)
	filePath := strings.TrimPrefix(chi.URLParam(r, "*"), "/")
	filePath, _ = url.PathUnescape(filePath)

	file, err := rc.Store.GetFile(r.Context(), filePath)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	symbols, err := rc.Store.ListSymbolsByFile(r.Context(), filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	inbound, outbound, err := rc.Store.ListFileEdgesForFile(r.Context(), filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	relatedTests, err := rc.Store.ListRelatedTestFiles(r.Context(), file.Dir, file.PackagePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	graph, err := s.fileLineageGraph(r.Context(), rc.Store, filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	source, err := s.fileSource(rc.Repo, filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render(w, governancePage(s.repoLayout(
		rc.Repo,
		"files",
		filepath.Base(file.Path),
		"",
		[]breadcrumbItem{{Label: "Catalog", Href: "/"}, {Label: rc.Repo.Name, Href: repoBaseHref(rc.Repo.ID)}, {Label: "Files", Href: repoFilesHref(rc.Repo.ID)}, {Label: file.Path}},
		fileHealthLabel(file),
		fileHealthTone(file),
	), fileDetailView(fileDetailData{
		RepoID:       rc.Repo.ID,
		Meta:         meta,
		File:         file,
		ActiveTab:    detailTabValue(r, "overview", "overview", "lineage", "source"),
		Symbols:      symbols,
		Inbound:      inbound,
		Outbound:     outbound,
		RelatedTests: relatedTests,
		Graph:        graph,
		Source:       source,
	})))
}

func (s *Server) packages(w http.ResponseWriter, r *http.Request) {
	rc, err := s.resolveRepo(r)
	if err != nil {
		s.writeRepoError(w, r, err)
		return
	}

	meta, _ := snapshotMeta(r.Context(), rc.Store)
	packagesList, err := rc.Store.ListPackages(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	graph, err := s.packageGraph(r.Context(), rc.Store, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	layout := s.repoLayout(
		rc.Repo,
		"packages",
		"Package Catalog",
		"",
		[]breadcrumbItem{{Label: "Catalog", Href: "/"}, {Label: rc.Repo.Name, Href: repoBaseHref(rc.Repo.ID)}, {Label: "Packages"}},
		strconv.Itoa(len(packagesList))+" packages",
		"neutral",
	)
	layout.MainClass = "overflow-hidden"
	layout.ContentClass = "h-full w-full"
	s.render(w, governancePage(layout, packagesView(packagesData{
		RepoID:   rc.Repo.ID,
		Meta:     meta,
		Packages: packagesList,
		Graph:    graph,
	})))
}

func (s *Server) packageDetail(w http.ResponseWriter, r *http.Request) {
	rc, err := s.resolveRepo(r)
	if err != nil {
		s.writeRepoError(w, r, err)
		return
	}

	meta, _ := snapshotMeta(r.Context(), rc.Store)
	packageKey := strings.TrimPrefix(chi.URLParam(r, "*"), "/")
	packageKey, _ = url.PathUnescape(packageKey)
	if cleanRoute := packageRoutePath(packageKey, meta); cleanRoute != packageKey {
		target := repoPackagesHref(rc.Repo.ID) + "/" + escapePathSegments(cleanRoute)
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently)
		return
	}

	item, err := s.packageFromRoute(r.Context(), rc.Store, meta, packageKey)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files, err := rc.Store.ListPackageFiles(r.Context(), item.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	neighborhood, err := rc.Store.ListPackageEdgeNeighborhood(r.Context(), item.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	inbound := make([]model.PackageEdge, 0)
	outbound := make([]model.PackageEdge, 0)
	for _, edge := range neighborhood {
		if edge.FromPath == item.Path {
			outbound = append(outbound, edge)
		}
		if edge.ToPath == item.Path {
			inbound = append(inbound, edge)
		}
	}
	graph, err := s.packageGraph(r.Context(), rc.Store, item.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render(w, governancePage(s.repoLayout(
		rc.Repo,
		"packages",
		packageDisplayPath(item),
		"",
		[]breadcrumbItem{{Label: "Catalog", Href: "/"}, {Label: rc.Repo.Name, Href: repoBaseHref(rc.Repo.ID)}, {Label: "Packages", Href: repoPackagesHref(rc.Repo.ID)}, {Label: packageDisplayPath(item)}},
		packageHealthLabel(item),
		packageHealthTone(item),
	), packageDetailView(packageDetailData{
		RepoID:    rc.Repo.ID,
		Meta:      meta,
		Package:   item,
		ActiveTab: detailTabValue(r, "overview", "overview", "neighborhood", "files"),
		Files:     files,
		Inbound:   inbound,
		Outbound:  outbound,
		Graph:     graph,
	})))
}

func (s *Server) packageFromRoute(ctx context.Context, st *store.Store, meta *model.SnapshotMeta, routePath string) (model.Package, error) {
	dir := packageRouteToDir(routePath)
	if meta != nil && strings.TrimSpace(meta.ModulePath) != "" {
		canonicalPath := meta.ModulePath
		if dir != "." && dir != "" {
			canonicalPath = path.Join(meta.ModulePath, dir)
		}
		item, err := st.GetPackage(ctx, canonicalPath)
		if err == nil {
			return item, nil
		}
		if err != sql.ErrNoRows {
			return model.Package{}, err
		}
	}
	return st.GetPackageByDir(ctx, dir)
}

func (s *Server) render(w http.ResponseWriter, page g.Node) {
	if err := render.HTML(w, page); err != nil {
		s.logger.Error("render page failed", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) resolveRepo(r *http.Request) (repoContext, error) {
	repoID := strings.TrimSpace(chi.URLParam(r, "repoID"))
	repo, ok := s.cfg.Repository(repoID)
	if !ok {
		return repoContext{}, sql.ErrNoRows
	}
	st, ok := s.runtime.Store(repoID)
	if !ok {
		return repoContext{}, fmt.Errorf("repository store unavailable: %s", repoID)
	}
	return repoContext{Repo: repo, Store: st}, nil
}

func (s *Server) writeRepoError(w http.ResponseWriter, r *http.Request, err error) {
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func detailTabValue(r *http.Request, fallback string, allowed ...string) string {
	value := strings.TrimSpace(r.URL.Query().Get("tab"))
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return fallback
}

func packageRouteToDir(routePath string) string {
	routePath = strings.Trim(strings.TrimSpace(routePath), "/")
	if routePath == "" || routePath == rootPackageRouteSegment {
		return "."
	}
	return routePath
}

func (s *Server) repoLayout(repo config.Repository, section string, title string, message string, breadcrumbs []breadcrumbItem, statusLabel string, statusTone string) layoutData {
	return layoutData{
		Title:        title,
		Message:      message,
		Section:      section,
		Breadcrumbs:  breadcrumbs,
		StatusLabel:  statusLabel,
		StatusTone:   statusTone,
		Repositories: s.cfg.Repositories,
		ActiveRepo:   &repo,
		RefreshPath:  repoRefreshHref(repo.ID),
	}
}

func snapshotMeta(ctx context.Context, st *store.Store) (*model.SnapshotMeta, error) {
	meta, err := st.GetSnapshotMeta(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &meta, nil
}

func latestRun(ctx context.Context, st *store.Store) (*model.Run, error) {
	runs, err := st.ListRuns(ctx, 1)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, nil
	}
	return &runs[0], nil
}

type graphNodeData struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	Subtitle string  `json:"subtitle,omitempty"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Tone     string  `json:"tone,omitempty"`
}

type graphEdgeData struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
}

type graphResponse struct {
	Title string          `json:"title"`
	Nodes []graphNodeData `json:"nodes"`
	Edges []graphEdgeData `json:"edges"`
}

type fileSourceResponse struct {
	Path string `json:"path"`
	Lang string `json:"lang"`
	Code string `json:"code"`
}

func (s *Server) packageGraph(ctx context.Context, st *store.Store, focus string) (graphResponse, error) {
	packagesList, err := st.ListPackages(ctx)
	if err != nil {
		return graphResponse{}, err
	}

	var edges []model.PackageEdge
	if focus == "" {
		edges, err = st.ListPackageEdges(ctx)
	} else {
		edges, err = st.ListPackageEdgeNeighborhood(ctx, focus)
	}
	if err != nil {
		return graphResponse{}, err
	}

	nodes, graphEdges := buildPackageGraph(packagesList, edges, focus)
	return graphResponse{
		Title: "Package DAG",
		Nodes: nodes,
		Edges: graphEdges,
	}, nil
}

func (s *Server) fileLineageGraph(ctx context.Context, st *store.Store, filePath string) (graphResponse, error) {
	file, err := st.GetFile(ctx, filePath)
	if err != nil {
		return graphResponse{}, err
	}

	inbound, outbound, err := st.ListFileEdgesForFile(ctx, filePath)
	if err != nil {
		return graphResponse{}, err
	}

	nodes, edges := buildFileGraph(file, inbound, outbound)
	return graphResponse{
		Title: "File lineage",
		Nodes: nodes,
		Edges: edges,
	}, nil
}

func (s *Server) fileSource(repo config.Repository, filePath string) (fileSourceResponse, error) {
	if filepath.Ext(filePath) != ".go" {
		return fileSourceResponse{}, fmt.Errorf("only Go source files are supported")
	}

	absPath := filepath.Join(repo.Root, filepath.FromSlash(filePath))
	rel, err := filepath.Rel(repo.Root, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fileSourceResponse{}, fmt.Errorf("invalid file path")
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return fileSourceResponse{}, err
	}

	return fileSourceResponse{
		Path: filePath,
		Lang: "go",
		Code: string(content),
	}, nil
}

func sortFiles(files []model.File, sortBy string) {
	sort.Slice(files, func(i, j int) bool {
		left, right := files[i], files[j]
		switch sortBy {
		case "coverage":
			return derefCoverage(left.CoveragePct) > derefCoverage(right.CoveragePct)
		case "fanin":
			return left.FanIn > right.FanIn
		case "fanout":
			return left.FanOut > right.FanOut
		default:
			if left.LOC == right.LOC {
				return left.Path < right.Path
			}
			return left.LOC > right.LOC
		}
	})
}
