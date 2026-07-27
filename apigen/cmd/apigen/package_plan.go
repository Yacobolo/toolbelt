package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type unmatchedNamespacePolicy string

const (
	unmatchedNamespaceDefault unmatchedNamespacePolicy = "default"
	unmatchedNamespaceError   unmatchedNamespacePolicy = "error"
)

type resolvedGoPackageOutput struct {
	Dir               string
	Package           string
	ServerFile        string
	RequestModelsFile string
}

type namespaceGoPackageOutput struct {
	Namespace string
	Output    resolvedGoPackageOutput
}

type goPackagePlan struct {
	Default   *resolvedGoPackageOutput
	Aggregate *resolvedGoPackageOutput
	Packages  []namespaceGoPackageOutput
	Unmatched unmatchedNamespacePolicy
}

func (spec *goOutputSpec) usesSinglePackageForm() bool {
	return spec != nil && (strings.TrimSpace(spec.Dir) != "" ||
		strings.TrimSpace(spec.Package) != "" ||
		strings.TrimSpace(spec.ServerFile) != "" ||
		strings.TrimSpace(spec.RequestModelsFile) != "")
}

func (spec *goOutputSpec) usesPackagePlanForm() bool {
	return spec != nil && (spec.Default != nil ||
		spec.Aggregate != nil ||
		len(spec.Packages) > 0 ||
		strings.TrimSpace(spec.Unmatched) != "")
}

func normalizeGoPackagePlan(spec goOutputSpec) (*goPackagePlan, error) {
	singlePackage := spec.usesSinglePackageForm()
	packagePlan := spec.usesPackagePlanForm()
	if singlePackage && packagePlan {
		return nil, fmt.Errorf("go_out cannot mix dir/package/file fields with default/aggregate/packages/unmatched")
	}
	if singlePackage {
		if strings.TrimSpace(spec.Dir) == "" {
			return nil, fmt.Errorf("go_out.dir is required")
		}
		if _, err := resolveGoPackageOutput("go_out", goPackageOutputSpec{
			Dir:               spec.Dir,
			Package:           spec.Package,
			ServerFile:        spec.ServerFile,
			RequestModelsFile: spec.RequestModelsFile,
		}); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if !packagePlan {
		return nil, fmt.Errorf("go_out must declare dir or a package plan")
	}
	if len(spec.Packages) == 0 {
		return nil, fmt.Errorf("go_out.packages must declare at least one namespace")
	}

	policy := unmatchedNamespacePolicy(strings.TrimSpace(spec.Unmatched))
	if policy != unmatchedNamespaceDefault && policy != unmatchedNamespaceError {
		return nil, fmt.Errorf("go_out.unmatched must be one of default or error")
	}
	if policy == unmatchedNamespaceDefault && spec.Default == nil {
		return nil, fmt.Errorf("go_out.unmatched=default requires go_out.default")
	}
	if policy == unmatchedNamespaceError && spec.Default != nil {
		return nil, fmt.Errorf("go_out.default requires go_out.unmatched=default")
	}

	plan := &goPackagePlan{Unmatched: policy}
	outputPackages := map[string]string{}
	if spec.Default != nil {
		output, err := resolveGoPackageOutput("go_out.default", *spec.Default)
		if err != nil {
			return nil, err
		}
		plan.Default = &output
		outputPackages[filepath.Clean(output.Dir)] = output.Package
	}

	namespaces := make([]string, 0, len(spec.Packages))
	normalizedNamespaces := make(map[string]string, len(spec.Packages))
	for authoredNamespace := range spec.Packages {
		namespace := strings.TrimSpace(authoredNamespace)
		if namespace == "" {
			return nil, fmt.Errorf("go_out.packages namespace is required")
		}
		if previous, exists := normalizedNamespaces[namespace]; exists {
			return nil, fmt.Errorf("go_out.packages namespaces %q and %q normalize to the same value", previous, authoredNamespace)
		}
		normalizedNamespaces[namespace] = authoredNamespace
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)

	for _, namespace := range namespaces {
		outputSpec := spec.Packages[normalizedNamespaces[namespace]]
		output, err := resolveGoPackageOutput(fmt.Sprintf("go_out.packages[%q]", namespace), outputSpec)
		if err != nil {
			return nil, err
		}
		dir := filepath.Clean(output.Dir)
		if packageName, exists := outputPackages[dir]; exists && packageName != output.Package {
			return nil, fmt.Errorf("go_out packages resolve to the same directory with different package names")
		}
		outputPackages[dir] = output.Package
		plan.Packages = append(plan.Packages, namespaceGoPackageOutput{
			Namespace: namespace,
			Output:    output,
		})
	}

	if spec.Aggregate != nil {
		output, err := resolveGoPackageOutput("go_out.aggregate", *spec.Aggregate)
		if err != nil {
			return nil, err
		}
		if _, exists := outputPackages[filepath.Clean(output.Dir)]; exists {
			return nil, fmt.Errorf("go_out.aggregate must use a directory separate from package outputs")
		}
		plan.Aggregate = &output
	}
	return plan, nil
}

func resolveGoPackageOutput(fieldName string, spec goPackageOutputSpec) (resolvedGoPackageOutput, error) {
	if strings.TrimSpace(spec.Dir) == "" {
		return resolvedGoPackageOutput{}, fmt.Errorf("%s.dir is required", fieldName)
	}
	packageName, err := inferOrValidateManifestPackage(fieldName, spec.Package, spec.Dir)
	if err != nil {
		return resolvedGoPackageOutput{}, err
	}
	return resolvedGoPackageOutput{
		Dir:               filepath.Clean(spec.Dir),
		Package:           packageName,
		ServerFile:        coalesceString(spec.ServerFile, "server.apigen.gen.go"),
		RequestModelsFile: coalesceString(spec.RequestModelsFile, "request_models.gen.go"),
	}, nil
}
