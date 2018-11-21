package pkg

import (
	"github.com/pkg/errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type Resolution struct {
	LocalPrefix  string
	IncludeTests bool
}

type Dependency struct {
	Path        string
	ImportValue string
}
type Binary struct {
	BinaryName   string
	MainFile     string
	Dependencies []Dependency
}

func Binaries(path string, resolution Resolution) []Binary {
	var binaries []Binary

	cache := make(map[string][]Dependency)

	filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() &&
			strings.HasSuffix(path, ".go") &&
			!strings.Contains(path, "vendor") &&
			!strings.Contains(path, ".git",
			) {
			b := hasMain(path, resolution, cache)
			if b != nil {
				binaries = append(binaries, *b)
			}
		}

		return nil
	})

	return binaries
}

// hasMain goes through a file and determines if it has a main entrypoint.
// if it does, finds all transitive local package dependencies
func hasMain(path string, resolution Resolution, cache map[string][]Dependency) *Binary {
	const main = "main"

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		panic(err)
	}

	hasMainMethod := false
	hasMainPackage := false

	var imports []string

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.File:
			for _, imp := range x.Imports {
				imports = append(imports, trimQuotes(imp.Path.Value))
			}
			hasMainPackage = x.Name.Name == main
		case *ast.FuncDecl:
			hasMainMethod = x.Name.Name == main
			return !hasMainMethod
		}
		return true
	})

	if hasMainMethod && hasMainPackage {
		allDependenciesSet := make(map[string]Dependency)
		var allDeps []Dependency

		// for each import resolve its transitive deps
		for _, imported := range imports {
			importedPackage := trimQuotes(imported)

			marked := make(map[string]struct{})
			for _, i := range resolveLocalDeps(importedPackage, cache, resolution, marked) {
				if _, ok := allDependenciesSet[i.ImportValue]; !ok {
					allDeps = append(allDeps, i)
				}

				allDependenciesSet[i.ImportValue] = i
			}
		}

		sort.SliceStable(allDeps, func(i, j int) bool {
			return allDeps[i].ImportValue < allDeps[j].ImportValue
		})

		return &Binary{
			BinaryName:   filepath.Base(filepath.Dir(path)),
			MainFile:     path,
			Dependencies: allDeps,
		}
	}

	return nil
}

func trimQuotes(imp string) string {
	return strings.Replace(imp, "\"", "", -1)
}

func resolveLocalDeps(
	importValue string,
	existingSet map[string][]Dependency,
	resolution Resolution,
	marked map[string]struct{},
) []Dependency {
	// if we're not a local package we can skip it
	if !isLocal(importValue, resolution.LocalPrefix) {
		return nil
	}

	// if we already processed this import just use the cached value
	if deps, ok := existingSet[importValue]; ok {
		return deps
	}

	pathDependency, err := asDependency(importValue)
	if err != nil {
		return nil
	}

	fset := token.NewFileSet()

	// parse the dir optionally skipping tests
	dir, err := parser.ParseDir(fset, pathDependency.Path, func(info os.FileInfo) bool {
		if strings.HasSuffix(info.Name(), "_test.go") && !resolution.IncludeTests {
			return false
		}

		return true
	}, 0)
	if err != nil {
		panic(err)
	}

	imports := []Dependency{*pathDependency}

	// walk the AST for each package in the directory
	for _, f := range dir {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.File:
				// for each import in a file resolve its dependency tree walk its dependencies
				for _, imp := range x.Imports {
					importedPackage := trimQuotes(imp.Path.Value)

					dependency, err := asDependency(importedPackage)

					if err == nil && isLocal(importedPackage, resolution.LocalPrefix) {
						requiresCache := false
						// make sure we track circular references if we have two packages in the same
						// package :why: that reference each other. pretty much only happens with test code
						if _, ok := marked[importedPackage]; ok {
							requiresCache = true
						} else {
							marked[importedPackage] = struct{}{}
							imports = append(imports, *dependency)
							imports = append(imports, resolveLocalDeps(
								dependency.ImportValue,
								existingSet,
								resolution,
								marked,
							)...)
						}

						// if we needed to wait for the recursion to complete
						// and pull from the cache, load it
						if requiresCache {
							imports = append(imports, existingSet[importedPackage]...)
						}
					}
				}

				return false
			}
			return true
		})
	}

	existingSet[importValue] = imports

	return imports
}

func isLocal(importValue string, prefix string) bool {
	return strings.HasPrefix(trimQuotes(importValue), prefix)
}

func asDependency(importValue string) (*Dependency, error) {
	sanitizedImport := trimQuotes(importValue)

	gopathfile := path.Join(os.Getenv("GOPATH"), "src", sanitizedImport)
	vendorFile := path.Join("vendor", sanitizedImport)

	var actualPath string
	if exists(vendorFile) {
		actualPath = vendorFile
	} else if exists(gopathfile) {
		actualPath = gopathfile
	} else {
		return nil, errors.New("Import value doesn't exist")
	}

	return &Dependency{
		Path:        actualPath,
		ImportValue: importValue,
	}, nil
}

func exists(file string) bool {
	_, err := os.Stat(file)

	return !os.IsNotExist(err)
}
