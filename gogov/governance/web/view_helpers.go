package web

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Yacobolo/toolbelt/gogov/governance/config"
	"github.com/Yacobolo/toolbelt/gogov/governance/model"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func fileLink(repoID string, path string) g.Node {
	return h.A(h.Class("font-medium underline"), h.Href(fileHref(repoID, path)), g.Text(path))
}

func packageLink(repoID string, modulePath string, packagePath string) g.Node {
	return h.A(h.Class("font-medium underline"), h.Href(packageHref(repoID, packagePath, &model.SnapshotMeta{ModulePath: modulePath})), g.Text(packageListLabel(packagePath, modulePath)))
}

func repoBaseHref(repoID string) string {
	return "/repos/" + pathEscape(repoID)
}

func repoRunsHref(repoID string) string {
	return repoBaseHref(repoID) + "/runs"
}

func repoFilesHref(repoID string) string {
	return repoBaseHref(repoID) + "/files"
}

func repoPackagesHref(repoID string) string {
	return repoBaseHref(repoID) + "/packages"
}

func repoRefreshHref(repoID string) string {
	return repoBaseHref(repoID) + "/refresh"
}

func fileHref(repoID string, path string) string {
	return repoFilesHref(repoID) + "/" + escapePathSegments(path)
}

func packageHref(repoID string, packagePath string, meta *model.SnapshotMeta) string {
	return repoPackagesHref(repoID) + "/" + escapePathSegments(packageRoutePath(packagePath, meta))
}

func detailTabHref(base string, tab string) string {
	values := url.Values{}
	values.Set("tab", tab)
	return base + "?" + values.Encode()
}

func pathEscape(value string) string {
	return url.PathEscape(value)
}

const rootPackageRouteSegment = "@root"

func escapePathSegments(value string) string {
	parts := strings.Split(value, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func packageRoutePath(packagePath string, meta *model.SnapshotMeta) string {
	if meta != nil {
		modulePath := strings.Trim(strings.TrimSpace(meta.ModulePath), "/")
		if modulePath != "" {
			trimmed := strings.TrimPrefix(packagePath, modulePath)
			trimmed = strings.TrimPrefix(trimmed, "/")
			if trimmed != "" {
				return trimmed
			}
		}
	}
	if packagePath == "" || packagePath == "." {
		return rootPackageRouteSegment
	}
	return packagePath
}

func packageDisplayPath(pkg model.Package) string {
	dir := strings.TrimSpace(pkg.Dir)
	if dir == "" || dir == "." {
		if pkg.Name != "" {
			return pkg.Name
		}
		return rootPackageRouteSegment
	}
	return dir
}

func packageListLabel(packagePath string, modulePath string) string {
	display := packageRoutePath(packagePath, &model.SnapshotMeta{ModulePath: modulePath})
	if display == rootPackageRouteSegment {
		return display
	}
	return strings.TrimPrefix(display, "./")
}

func repoDisplayPath(repo config.Repository) string {
	if strings.TrimSpace(repo.SourcePath) != "" {
		return repo.SourcePath
	}
	return repo.Root
}

func coverageText(value *float64) string {
	if value == nil {
		return "n/a"
	}
	return strconv.FormatFloat(*value, 'f', 1, 64) + "%"
}

func formatTimeMeta(meta *model.SnapshotMeta) string {
	if meta == nil {
		return "Never"
	}
	return formatTimeValue(meta.RefreshedAt)
}

func formatTimeValue(value time.Time) string {
	return value.Local().Format("2006-01-02 15:04:05")
}

func modulePath(meta *model.SnapshotMeta) string {
	if meta == nil {
		return ""
	}
	return meta.ModulePath
}

func shortPkg(value string) string {
	parts := strings.Split(value, "/")
	if len(parts) <= 3 {
		return value
	}
	return strings.Join(parts[len(parts)-3:], "/")
}

func snapshotStatusLabel(meta *model.SnapshotMeta) string {
	if meta == nil {
		return "No snapshot yet"
	}
	switch meta.CoverageStatus {
	case model.CoverageStatusAvailable:
		return "Snapshot ready"
	case model.CoverageStatusPending:
		return "Coverage pending"
	case model.CoverageStatusFailed:
		return "Snapshot partial"
	case model.CoverageStatusMissing:
		return "Coverage missing"
	default:
		return "Snapshot captured"
	}
}

func snapshotStatusTone(meta *model.SnapshotMeta) string {
	if meta == nil {
		return "neutral"
	}
	switch meta.CoverageStatus {
	case model.CoverageStatusAvailable:
		return "good"
	case model.CoverageStatusPending, model.CoverageStatusMissing:
		return "warn"
	case model.CoverageStatusFailed:
		return "critical"
	default:
		return "neutral"
	}
}

func fileHealthLabel(file model.File) string {
	switch fileHealthTone(file) {
	case "critical":
		return "Needs review"
	case "warn":
		return "Watch file"
	default:
		return "Stable file"
	}
}

func fileHealthTone(file model.File) string {
	coverage := derefCoverage(file.CoveragePct)
	switch {
	case file.LOC >= 1500 || (file.CoveragePct != nil && coverage < 35):
		return "critical"
	case file.LOC >= 700 || file.FanOut >= 12 || (file.CoveragePct != nil && coverage < 65):
		return "warn"
	default:
		return "good"
	}
}

func packageHealthLabel(pkg model.Package) string {
	switch packageHealthTone(pkg) {
	case "critical":
		return "Boundary risk"
	case "warn":
		return "Watch package"
	default:
		return "Healthy package"
	}
}

func packageHealthTone(pkg model.Package) string {
	switch {
	case pkg.LOC >= 5000:
		return "critical"
	case pkg.LOC >= 2000 || pkg.ImportsCount >= 12 || pkg.ImportedByCount >= 12:
		return "warn"
	default:
		return "good"
	}
}

func firstOrFallback(meta *model.SnapshotMeta, selector func(model.SnapshotMeta) string, fallback string) string {
	if meta == nil {
		return fallback
	}
	return selector(*meta)
}

func firstOrZero(meta *model.SnapshotMeta, selector func(model.SnapshotMeta) int) int {
	if meta == nil {
		return 0
	}
	return selector(*meta)
}
