package web

import (
	"sort"
	"strconv"

	"github.com/Yacobolo/toolbelt/gogov/governance/model"
)

func buildPackageGraph(packagesList []model.Package, edges []model.PackageEdge, focus string) ([]graphNodeData, []graphEdgeData) {
	pkgMap := make(map[string]model.Package, len(packagesList))
	for _, item := range packagesList {
		pkgMap[item.Path] = item
	}

	nodes := make([]graphNodeData, 0, len(packagesList))
	graphEdges := make([]graphEdgeData, 0, len(edges))
	seen := make(map[string]struct{}, len(packagesList))

	orderedPaths := make([]string, 0, len(packagesList))
	for _, item := range packagesList {
		if focus != "" && item.Path != focus {
			if !connectedToFocus(item.Path, edges, focus) {
				continue
			}
		}
		orderedPaths = append(orderedPaths, item.Path)
	}
	sort.Strings(orderedPaths)

	for index, path := range orderedPaths {
		item := pkgMap[path]
		tone := ""
		if path == focus {
			tone = "focus"
		}
		nodes = append(nodes, graphNodeData{
			ID:       item.Path,
			Label:    shortPkg(item.Path),
			Subtitle: strconv.Itoa(item.FileCount) + " files · " + strconv.Itoa(item.LOC) + " LOC",
			X:        float64((index % 3) * 320),
			Y:        float64((index / 3) * 120),
			Tone:     tone,
		})
		seen[item.Path] = struct{}{}
	}

	for _, edge := range edges {
		if _, ok := seen[edge.FromPath]; !ok {
			continue
		}
		if _, ok := seen[edge.ToPath]; !ok {
			continue
		}
		graphEdges = append(graphEdges, graphEdgeData{
			ID:     edge.FromPath + "->" + edge.ToPath,
			Source: edge.FromPath,
			Target: edge.ToPath,
			Label:  strconv.Itoa(edge.Weight),
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
