package web

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Yacobolo/toolbelt/gogov/governance/model"
)

func buildPackageGraph(packagesList []model.Package, edges []model.PackageEdge, focus string) ([]graphNodeData, []graphEdgeData) {
	pkgMap := make(map[string]model.Package, len(packagesList))
	for _, item := range packagesList {
		pkgMap[item.Path] = item
	}

	visiblePaths := make([]string, 0, len(packagesList))
	for _, item := range packagesList {
		if focus != "" && item.Path != focus {
			if !connectedToFocus(item.Path, edges, focus) {
				continue
			}
		}
		visiblePaths = append(visiblePaths, item.Path)
	}

	sort.Strings(visiblePaths)

	visibleSet := make(map[string]struct{}, len(visiblePaths))
	for _, path := range visiblePaths {
		visibleSet[path] = struct{}{}
	}

	visibleEdges := make([]model.PackageEdge, 0, len(edges))
	for _, edge := range edges {
		if _, ok := visibleSet[edge.FromPath]; !ok {
			continue
		}
		if _, ok := visibleSet[edge.ToPath]; !ok {
			continue
		}
		visibleEdges = append(visibleEdges, edge)
	}

	var (
		positions    map[string]graphPoint
		orderedPaths []string
	)
	if focus == "" {
		positions, orderedPaths = layoutOverviewPackageGraph(visiblePaths, visibleEdges, pkgMap)
	} else {
		positions, orderedPaths = layoutFocusedPackageGraph(visiblePaths, visibleEdges, pkgMap, focus)
	}

	nodes := make([]graphNodeData, 0, len(visiblePaths))
	for _, path := range orderedPaths {
		item := pkgMap[path]
		point := positions[path]
		tone := packageLane(path).Tone
		if path == focus {
			tone = "focus"
		}
		nodes = append(nodes, graphNodeData{
			ID:       item.Path,
			Label:    shortPkg(item.Path),
			Subtitle: strconv.Itoa(item.FileCount) + " files · " + strconv.Itoa(item.LOC) + " LOC",
			X:        point.X,
			Y:        point.Y,
			Tone:     tone,
		})
	}

	graphEdges := make([]graphEdgeData, 0, len(visibleEdges))
	for _, edge := range visibleEdges {
		graphEdges = append(graphEdges, graphEdgeData{
			ID:     edge.FromPath + "->" + edge.ToPath,
			Source: edge.FromPath,
			Target: edge.ToPath,
		})
	}

	return nodes, graphEdges
}

func connectedToFocus(path string, edges []model.PackageEdge, focus string) bool {
	if path == focus {
		return true
	}
	for _, edge := range edges {
		if edge.FromPath == focus && edge.ToPath == path {
			return true
		}
		if edge.ToPath == focus && edge.FromPath == path {
			return true
		}
	}
	return false
}

func buildFileGraph(file model.File, inbound []model.FileEdge, outbound []model.FileEdge) ([]graphNodeData, []graphEdgeData) {
	nodes := []graphNodeData{{
		ID:       file.Path,
		Label:    shortPkg(file.Path),
		Subtitle: file.Path,
		X:        0,
		Y:        0,
		Tone:     "focus",
	}}
	graphEdges := make([]graphEdgeData, 0, len(inbound)+len(outbound))
	seen := map[string]struct{}{file.Path: {}}

	for index, edge := range inbound {
		path := edge.FromPath
		if _, ok := seen[path]; !ok {
			nodes = append(nodes, graphNodeData{
				ID:       path,
				Label:    shortPkg(path),
				Subtitle: path,
				X:        -320,
				Y:        float64(index * 110),
			})
			seen[path] = struct{}{}
		}
		graphEdges = append(graphEdges, graphEdgeData{
			ID:     path + "->" + file.Path + "::" + edge.Kind,
			Source: path,
			Target: file.Path,
			Label:  edge.Kind,
		})
	}

	for index, edge := range outbound {
		path := edge.ToPath
		if _, ok := seen[path]; !ok {
			nodes = append(nodes, graphNodeData{
				ID:       path,
				Label:    shortPkg(path),
				Subtitle: path,
				X:        320,
				Y:        float64(index * 110),
			})
			seen[path] = struct{}{}
		}
		graphEdges = append(graphEdges, graphEdgeData{
			ID:     file.Path + "->" + path + "::" + edge.Kind,
			Source: file.Path,
			Target: path,
			Label:  edge.Kind,
		})
	}

	return nodes, graphEdges
}

func derefCoverage(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

type graphPoint struct {
	X float64
	Y float64
}

type packageLaneMeta struct {
	Order int
	Tone  string
}

type layoutNode struct {
	Path string
	Rank int
	Lane packageLaneMeta
}

func layoutOverviewPackageGraph(paths []string, edges []model.PackageEdge, packagesByPath map[string]model.Package) (map[string]graphPoint, []string) {
	ranks := computePackageRanks(paths, edges)
	grouped := make(map[int]map[int][]layoutNode)
	laneMaxCount := make(map[int]int)
	maxRank := 0

	for _, path := range paths {
		lane := packageLane(path)
		rank := ranks[path]
		if rank > maxRank {
			maxRank = rank
		}
		if grouped[lane.Order] == nil {
			grouped[lane.Order] = make(map[int][]layoutNode)
		}
		grouped[lane.Order][rank] = append(grouped[lane.Order][rank], layoutNode{
			Path: path,
			Rank: rank,
			Lane: lane,
		})
	}

	for laneOrder, byRank := range grouped {
		maxCount := 1
		for rank := 0; rank <= maxRank; rank++ {
			nodes := byRank[rank]
			sortLayoutNodes(nodes, packagesByPath)
			byRank[rank] = nodes
			if len(nodes) > maxCount {
				maxCount = len(nodes)
			}
		}
		laneMaxCount[laneOrder] = maxCount
	}

	const (
		columnGap   = 272.0
		rowGap      = 42.0
		lanePadding = 20.0
		nodeGap     = 68.0
	)

	laneOffsets := make(map[int]float64, len(grouped))
	currentY := 0.0
	laneOrders := orderedLaneOrders(grouped)
	for _, laneOrder := range laneOrders {
		laneOffsets[laneOrder] = currentY
		laneHeight := lanePadding*2 + nodeGap*float64(max(0, laneMaxCount[laneOrder]-1))
		currentY += laneHeight + rowGap
	}

	positions := make(map[string]graphPoint, len(paths))
	ordered := make([]layoutNode, 0, len(paths))
	for _, laneOrder := range laneOrders {
		byRank := grouped[laneOrder]
		laneHeight := lanePadding*2 + nodeGap*float64(max(0, laneMaxCount[laneOrder]-1))
		laneTop := laneOffsets[laneOrder]
		for rank := 0; rank <= maxRank; rank++ {
			nodes := byRank[rank]
			groupHeight := nodeGap * float64(max(0, len(nodes)-1))
			startY := laneTop + (laneHeight-groupHeight)/2
			for index, node := range nodes {
				positions[node.Path] = graphPoint{
					X: float64(rank) * columnGap,
					Y: startY + float64(index)*nodeGap,
				}
				ordered = append(ordered, node)
			}
		}
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Rank != ordered[j].Rank {
			return ordered[i].Rank < ordered[j].Rank
		}
		if ordered[i].Lane.Order != ordered[j].Lane.Order {
			return ordered[i].Lane.Order < ordered[j].Lane.Order
		}
		left := positions[ordered[i].Path]
		right := positions[ordered[j].Path]
		if left.Y != right.Y {
			return left.Y < right.Y
		}
		return ordered[i].Path < ordered[j].Path
	})

	orderedPaths := make([]string, 0, len(ordered))
	for _, node := range ordered {
		orderedPaths = append(orderedPaths, node.Path)
	}

	return positions, orderedPaths
}

func layoutFocusedPackageGraph(paths []string, edges []model.PackageEdge, packagesByPath map[string]model.Package, focus string) (map[string]graphPoint, []string) {
	positions := make(map[string]graphPoint, len(paths))
	positions[focus] = graphPoint{X: 0, Y: 0}

	var inbound []string
	var outbound []string
	for _, edge := range edges {
		switch {
		case edge.ToPath == focus:
			inbound = append(inbound, edge.FromPath)
		case edge.FromPath == focus:
			outbound = append(outbound, edge.ToPath)
		}
	}

	sortPackagePathsByImportance(inbound, packagesByPath)
	sortPackagePathsByImportance(outbound, packagesByPath)

	positionColumn(inbound, positions, -340, 94)
	positionColumn(outbound, positions, 340, 94)

	ordered := make([]string, 0, len(paths))
	ordered = append(ordered, inbound...)
	ordered = append(ordered, focus)
	ordered = append(ordered, outbound...)
	return positions, ordered
}

func computePackageRanks(paths []string, edges []model.PackageEdge) map[string]int {
	indegree := make(map[string]int, len(paths))
	outgoing := make(map[string][]string, len(paths))
	for _, path := range paths {
		indegree[path] = 0
	}

	for _, edge := range edges {
		outgoing[edge.FromPath] = append(outgoing[edge.FromPath], edge.ToPath)
		indegree[edge.ToPath]++
	}

	for path := range outgoing {
		sort.Strings(outgoing[path])
	}

	queue := make([]string, 0, len(paths))
	for _, path := range paths {
		if indegree[path] == 0 {
			queue = append(queue, path)
		}
	}
	sort.Strings(queue)

	ranks := make(map[string]int, len(paths))
	processed := make(map[string]struct{}, len(paths))

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		processed[current] = struct{}{}

		for _, next := range outgoing[current] {
			if ranks[next] < ranks[current]+1 {
				ranks[next] = ranks[current] + 1
			}
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
				sort.Strings(queue)
			}
		}
	}

	if len(processed) != len(paths) {
		for _, path := range paths {
			if _, ok := processed[path]; !ok {
				ranks[path] = 0
			}
		}
	}

	return ranks
}

func orderedLaneOrders(grouped map[int]map[int][]layoutNode) []int {
	laneOrders := make([]int, 0, len(grouped))
	for laneOrder := range grouped {
		laneOrders = append(laneOrders, laneOrder)
	}
	sort.Ints(laneOrders)
	return laneOrders
}

func sortLayoutNodes(nodes []layoutNode, packagesByPath map[string]model.Package) {
	sort.SliceStable(nodes, func(i, j int) bool {
		left := packagesByPath[nodes[i].Path]
		right := packagesByPath[nodes[j].Path]
		if left.ImportedByCount != right.ImportedByCount {
			return left.ImportedByCount > right.ImportedByCount
		}
		if left.ImportsCount != right.ImportsCount {
			return left.ImportsCount > right.ImportsCount
		}
		return nodes[i].Path < nodes[j].Path
	})
}

func sortPackagePathsByImportance(paths []string, packagesByPath map[string]model.Package) {
	sort.SliceStable(paths, func(i, j int) bool {
		left := packagesByPath[paths[i]]
		right := packagesByPath[paths[j]]
		if left.ImportedByCount != right.ImportedByCount {
			return left.ImportedByCount > right.ImportedByCount
		}
		if left.ImportsCount != right.ImportsCount {
			return left.ImportsCount > right.ImportsCount
		}
		return paths[i] < paths[j]
	})
}

func positionColumn(paths []string, positions map[string]graphPoint, x float64, gap float64) {
	startY := -gap * float64(len(paths)-1) / 2
	for index, path := range paths {
		positions[path] = graphPoint{
			X: x,
			Y: startY + float64(index)*gap,
		}
	}
}

func packageLane(path string) packageLaneMeta {
	relative := packageArchitecturePath(path)

	switch {
	case strings.HasPrefix(relative, "cmd/"):
		return packageLaneMeta{Order: 0, Tone: "entry"}
	case strings.HasPrefix(relative, "internal/ui/"),
		strings.HasPrefix(relative, "internal/api"),
		strings.HasPrefix(relative, "internal/platform/router"),
		strings.HasPrefix(relative, "internal/platform/render"),
		strings.HasPrefix(relative, "internal/platform/ds"),
		strings.HasPrefix(relative, "web/resources"):
		return packageLaneMeta{Order: 1, Tone: "delivery"}
	case strings.HasPrefix(relative, "internal/app"),
		strings.HasPrefix(relative, "internal/services"),
		strings.HasPrefix(relative, "internal/agentplatform"),
		strings.HasPrefix(relative, "internal/agentruntime"):
		return packageLaneMeta{Order: 2, Tone: "application"}
	case strings.HasPrefix(relative, "internal/domain/"),
		strings.HasPrefix(relative, "internal/runtimecontracts"):
		return packageLaneMeta{Order: 3, Tone: "domain"}
	case strings.HasPrefix(relative, "internal/config"),
		strings.HasPrefix(relative, "internal/store"),
		strings.HasPrefix(relative, "internal/platform/"),
		strings.HasPrefix(relative, "sandbox/"):
		return packageLaneMeta{Order: 4, Tone: "foundation"}
	default:
		return packageLaneMeta{Order: 5, Tone: "support"}
	}
}

func packageArchitecturePath(path string) string {
	for _, marker := range []string{"/cmd/", "/internal/", "/web/", "/sandbox/"} {
		if index := strings.Index(path, marker); index >= 0 {
			return path[index+1:]
		}
	}
	return path
}

func max(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
