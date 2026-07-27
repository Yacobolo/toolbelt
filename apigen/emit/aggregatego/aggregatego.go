// Package aggregatego emits optional Go route composition for independently
// generated APIGen server packages.
package aggregatego

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"
)

// ServerPackage describes one generated APIGen server package.
type ServerPackage struct {
	// Name is a stable, language-level logical name used for exported fields.
	Name string
	// ImportPath is the canonical Go import path of the generated server.
	ImportPath string
	// PackageName is the declared Go package name of the generated server.
	PackageName string
}

// Options configures aggregate Go route composition.
type Options struct {
	PackageName string
	Packages    []ServerPackage
}

type resolvedServerPackage struct {
	ServerPackage
	Alias string
	Field string
}

const chiRuntimeImportPath = "github.com/Yacobolo/toolbelt/apigen/runtime/chi"

var goKeywords = map[string]struct{}{
	"break": {}, "default": {}, "func": {}, "interface": {}, "select": {},
	"case": {}, "defer": {}, "go": {}, "map": {}, "struct": {},
	"chan": {}, "else": {}, "goto": {}, "package": {}, "switch": {},
	"const": {}, "fallthrough": {}, "if": {}, "range": {}, "type": {},
	"continue": {}, "for": {}, "import": {}, "return": {}, "var": {},
}

// Emit renders typed loose and strict route composition for generated servers.
func Emit(opts Options) ([]byte, error) {
	if !validGoIdentifier(opts.PackageName) {
		return nil, fmt.Errorf("invalid aggregate Go package %q", opts.PackageName)
	}
	if len(opts.Packages) == 0 {
		return nil, fmt.Errorf("aggregate requires at least one server package")
	}
	packages, err := resolveServerPackages(opts.Packages)
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", opts.PackageName)
	b.WriteString("import (\n")
	fmt.Fprintf(&b, "\tapigenchi %q\n", chiRuntimeImportPath)
	for _, serverPackage := range packages {
		fmt.Fprintf(&b, "\t%s %q\n", serverPackage.Alias, serverPackage.ImportPath)
	}
	b.WriteString(")\n\n")

	b.WriteString("// Servers contains one loose generated server per composed package.\n")
	b.WriteString("type Servers struct {\n")
	for _, serverPackage := range packages {
		fmt.Fprintf(&b, "\t%s %s.GenServerInterface\n", serverPackage.Field, serverPackage.Alias)
	}
	b.WriteString("}\n\n")

	b.WriteString("// RegisterAPIGenRoutes mounts every composed generated server.\n")
	b.WriteString("func RegisterAPIGenRoutes(router apigenchi.Router, servers Servers) {\n")
	for _, serverPackage := range packages {
		fmt.Fprintf(&b, "\t%s.RegisterAPIGenRoutes(router, servers.%s)\n", serverPackage.Alias, serverPackage.Field)
	}
	b.WriteString("}\n\n")

	b.WriteString("// StrictServers contains one strict generated handler per composed package.\n")
	b.WriteString("type StrictServers struct {\n")
	for _, serverPackage := range packages {
		fmt.Fprintf(&b, "\t%s %s.GenStrictServerInterface\n", serverPackage.Field, serverPackage.Alias)
	}
	b.WriteString("}\n\n")

	b.WriteString("// TransportErrorResponders contains one transport error responder per composed package.\n")
	b.WriteString("type TransportErrorResponders struct {\n")
	for _, serverPackage := range packages {
		fmt.Fprintf(&b, "\t%s %s.GenTransportErrorResponder\n", serverPackage.Field, serverPackage.Alias)
	}
	b.WriteString("}\n\n")

	b.WriteString("// RegisterAPIGenStrictRoutes mounts every composed strict generated server.\n")
	b.WriteString("func RegisterAPIGenStrictRoutes(router apigenchi.Router, servers StrictServers, responders TransportErrorResponders) {\n")
	for _, serverPackage := range packages {
		fmt.Fprintf(
			&b,
			"\t%s.RegisterAPIGenStrictRoutes(router, servers.%s, responders.%s)\n",
			serverPackage.Alias,
			serverPackage.Field,
			serverPackage.Field,
		)
	}
	b.WriteString("}\n")
	return []byte(b.String()), nil
}

func resolveServerPackages(authored []ServerPackage) ([]resolvedServerPackage, error) {
	packages := append([]ServerPackage(nil), authored...)
	sort.Slice(packages, func(left, right int) bool {
		if packages[left].ImportPath != packages[right].ImportPath {
			return packages[left].ImportPath < packages[right].ImportPath
		}
		if packages[left].PackageName != packages[right].PackageName {
			return packages[left].PackageName < packages[right].PackageName
		}
		return packages[left].Name < packages[right].Name
	})

	packageCounts := map[string]int{}
	fieldCounts := map[string]int{}
	fields := make([]string, len(packages))
	importPaths := map[string]struct{}{}
	for index, serverPackage := range packages {
		if strings.TrimSpace(serverPackage.Name) == "" {
			return nil, fmt.Errorf("aggregate server package name is required")
		}
		if !canonicalGoImportPath(serverPackage.ImportPath) {
			return nil, fmt.Errorf("aggregate server package %q requires a canonical Go import path", serverPackage.Name)
		}
		if serverPackage.ImportPath == chiRuntimeImportPath {
			return nil, fmt.Errorf("aggregate server package %q conflicts with the APIGen Chi runtime import", serverPackage.Name)
		}
		if !validGoIdentifier(serverPackage.PackageName) {
			return nil, fmt.Errorf("aggregate server package %q has invalid Go package %q", serverPackage.Name, serverPackage.PackageName)
		}
		if _, exists := importPaths[serverPackage.ImportPath]; exists {
			return nil, fmt.Errorf("aggregate server import path %q is declared more than once", serverPackage.ImportPath)
		}
		importPaths[serverPackage.ImportPath] = struct{}{}
		field := exportedIdentifier(serverPackage.Name)
		fields[index] = field
		packageCounts[serverPackage.PackageName]++
		fieldCounts[field]++
	}

	usedAliases := map[string]string{"apigenchi": chiRuntimeImportPath}
	usedFields := map[string]string{}
	for index, serverPackage := range packages {
		if packageCounts[serverPackage.PackageName] == 1 && serverPackage.PackageName != "apigenchi" {
			usedAliases[serverPackage.PackageName] = serverPackage.ImportPath
		}
		if fieldCounts[fields[index]] == 1 {
			usedFields[fields[index]] = serverPackage.ImportPath
		}
	}
	resolved := make([]resolvedServerPackage, 0, len(packages))
	for index, serverPackage := range packages {
		alias := serverPackage.PackageName
		if packageCounts[serverPackage.PackageName] > 1 || alias == "apigenchi" {
			var err error
			alias, err = allocateHashedIdentifier(alias+"_", serverPackage.ImportPath, false, usedAliases)
			if err != nil {
				return nil, err
			}
		}
		usedAliases[alias] = serverPackage.ImportPath

		field := fields[index]
		if fieldCounts[field] > 1 {
			var err error
			field, err = allocateHashedIdentifier(field, serverPackage.ImportPath, true, usedFields)
			if err != nil {
				return nil, err
			}
		}
		usedFields[field] = serverPackage.ImportPath
		resolved = append(resolved, resolvedServerPackage{
			ServerPackage: serverPackage,
			Alias:         alias,
			Field:         field,
		})
	}
	return resolved, nil
}

func allocateHashedIdentifier(prefix, identity string, uppercase bool, used map[string]string) (string, error) {
	sum := sha256.Sum256([]byte(identity))
	digest := hex.EncodeToString(sum[:])
	if uppercase {
		digest = strings.ToUpper(digest)
	}
	for length := 8; length <= len(digest); length += 2 {
		candidate := prefix + digest[:length]
		if previous, exists := used[candidate]; !exists || previous == identity {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cannot allocate a unique aggregate Go identifier for %q", identity)
}

func canonicalGoImportPath(importPath string) bool {
	if !(importPath != "" &&
		strings.TrimSpace(importPath) == importPath &&
		importPath != "." &&
		importPath != ".." &&
		!strings.HasPrefix(importPath, "../") &&
		!path.IsAbs(importPath) &&
		!strings.Contains(importPath, `\`) &&
		!strings.HasSuffix(importPath, "/") &&
		path.Clean(importPath) == importPath) {
		return false
	}
	for _, character := range importPath {
		if character == '/' || unicode.IsLetter(character) || unicode.IsDigit(character) ||
			strings.ContainsRune("-._~+", character) {
			continue
		}
		return false
	}
	return true
}

func validGoIdentifier(value string) bool {
	if value == "" || value == "_" {
		return false
	}
	if _, keyword := goKeywords[value]; keyword {
		return false
	}
	for index, character := range value {
		if character == '_' || unicode.IsLetter(character) || (index > 0 && unicode.IsDigit(character)) {
			continue
		}
		return false
	}
	return true
}

func exportedIdentifier(value string) string {
	var out []rune
	upperNext := true
	for _, character := range value {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			upperNext = true
			continue
		}
		if len(out) == 0 && unicode.IsDigit(character) {
			out = append(out, []rune("Package")...)
		}
		if upperNext {
			character = unicode.ToUpper(character)
			upperNext = false
		}
		out = append(out, character)
	}
	if len(out) == 0 {
		return "Package"
	}
	return string(out)
}
