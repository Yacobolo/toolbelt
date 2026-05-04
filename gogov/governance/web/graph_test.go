package web

import (
	"testing"

	"github.com/Yacobolo/toolbelt/gogov/governance/model"
)

func TestBuildPackageGraphOverviewUsesDependencyLayers(t *testing.T) {
	packagesList := []model.Package{
		{Path: "github.com/acme/project/cmd/server", FileCount: 1, LOC: 10},
		{Path: "github.com/acme/project/internal/ui/agents", FileCount: 2, LOC: 20, ImportedByCount: 1, ImportsCount: 2},
		{Path: "github.com/acme/project/internal/app", FileCount: 3, LOC: 30, ImportedByCount: 2, ImportsCount: 2},
		{Path: "github.com/acme/project/internal/domain/agents", FileCount: 1, LOC: 15, ImportedByCount: 1},
		{Path: "github.com/acme/project/internal/platform/storage", FileCount: 1, LOC: 12, ImportedByCount: 1},
	}
	edges := []model.PackageEdge{
		{FromPath: "github.com/acme/project/cmd/server", ToPath: "github.com/acme/project/internal/ui/agents", Weight: 1},
		{FromPath: "github.com/acme/project/internal/ui/agents", ToPath: "github.com/acme/project/internal/app", Weight: 1},
		{FromPath: "github.com/acme/project/internal/app", ToPath: "github.com/acme/project/internal/domain/agents", Weight: 1},
		{FromPath: "github.com/acme/project/internal/app", ToPath: "github.com/acme/project/internal/platform/storage", Weight: 1},
	}

	nodes, _ := buildPackageGraph(packagesList, edges, "")
	positions := nodePositions(nodes)

	if positions["github.com/acme/project/cmd/server"].X >= positions["github.com/acme/project/internal/ui/agents"].X {
		t.Fatalf("expected cmd package to appear before UI package")
	}
	if positions["github.com/acme/project/internal/ui/agents"].X >= positions["github.com/acme/project/internal/app"].X {
		t.Fatalf("expected UI package to appear before app package")
	}
	if positions["github.com/acme/project/internal/app"].X >= positions["github.com/acme/project/internal/domain/agents"].X {
		t.Fatalf("expected app package to appear before domain package")
	}
	if positions["github.com/acme/project/internal/app"].X >= positions["github.com/acme/project/internal/platform/storage"].X {
		t.Fatalf("expected app package to appear before foundation package")
	}

	if positions["github.com/acme/project/cmd/server"].Y >= positions["github.com/acme/project/internal/ui/agents"].Y {
		t.Fatalf("expected entry lane to sit above delivery lane")
	}
	if positions["github.com/acme/project/internal/ui/agents"].Y >= positions["github.com/acme/project/internal/app"].Y {
		t.Fatalf("expected delivery lane to sit above application lane")
	}
	if positions["github.com/acme/project/internal/app"].Y >= positions["github.com/acme/project/internal/domain/agents"].Y {
		t.Fatalf("expected application lane to sit above domain lane")
	}
	if positions["github.com/acme/project/internal/domain/agents"].Y >= positions["github.com/acme/project/internal/platform/storage"].Y {
		t.Fatalf("expected domain lane to sit above foundation lane")
	}
}

func TestBuildPackageGraphFocusCentersSelectedPackage(t *testing.T) {
	focus := "github.com/acme/project/internal/app"
	packagesList := []model.Package{
		{Path: "github.com/acme/project/cmd/server", ImportedByCount: 1},
		{Path: "github.com/acme/project/internal/ui/agents", ImportedByCount: 1},
		{Path: focus, ImportedByCount: 2, ImportsCount: 2},
		{Path: "github.com/acme/project/internal/domain/agents"},
		{Path: "github.com/acme/project/internal/platform/storage"},
	}
	edges := []model.PackageEdge{
		{FromPath: "github.com/acme/project/cmd/server", ToPath: focus, Weight: 1},
		{FromPath: "github.com/acme/project/internal/ui/agents", ToPath: focus, Weight: 1},
		{FromPath: focus, ToPath: "github.com/acme/project/internal/domain/agents", Weight: 1},
		{FromPath: focus, ToPath: "github.com/acme/project/internal/platform/storage", Weight: 1},
	}

	nodes, _ := buildPackageGraph(packagesList, edges, focus)
	positions := nodePositions(nodes)

	if positions[focus].X != 0 || positions[focus].Y != 0 {
		t.Fatalf("expected focused package to stay centered, got (%v, %v)", positions[focus].X, positions[focus].Y)
	}
	if positions["github.com/acme/project/cmd/server"].X >= 0 {
		t.Fatalf("expected dependents to appear left of the focus")
	}
	if positions["github.com/acme/project/internal/ui/agents"].X >= 0 {
		t.Fatalf("expected dependents to appear left of the focus")
	}
	if positions["github.com/acme/project/internal/domain/agents"].X <= 0 {
		t.Fatalf("expected dependencies to appear right of the focus")
	}
	if positions["github.com/acme/project/internal/platform/storage"].X <= 0 {
		t.Fatalf("expected dependencies to appear right of the focus")
	}
}

func nodePositions(nodes []graphNodeData) map[string]graphPoint {
	positions := make(map[string]graphPoint, len(nodes))
	for _, node := range nodes {
		positions[node.ID] = graphPoint{X: node.X, Y: node.Y}
	}
	return positions
}
