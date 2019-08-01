package pkg

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
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
	Dependencies []string
}

func Binaries(path string, resolution Resolution) []Binary {
	var binaries []Binary

	var l sync.Mutex
	var s sync.WaitGroup
	markedPackages := make(map[string]struct{})
	_ = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() &&
			strings.HasSuffix(path, ".go") &&
			!strings.Contains(path, "vendor") &&
			!strings.Contains(path, ".git",
			) {
			dir := filepath.Dir(path)
			if _, ok := markedPackages[dir]; !ok {
				markedPackages[dir] = struct{}{}

				data, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}

				if strings.Contains(string(data), "package main") {
					s.Add(1)
					go func() {
						data, err := exec.Command("go", "list", "-f", `{{ join .Deps  "\n"}}`, path).Output()
						if err != nil {
							logrus.Error(err)
							return
						}

						packages := strings.Split(string(data), "\n")

						var filtered []string

						for _, pkg := range packages {
							if strings.HasPrefix(pkg, resolution.LocalPrefix) && !strings.HasPrefix(pkg, resolution.LocalPrefix+"/vendor") {
								filtered = append(filtered, pkg)
							}
						}

						logrus.Debugf("found main %s", dir)

						l.Lock()
						binaries = append(binaries, Binary{
							BinaryName:   filepath.Base(dir),
							MainFile:     path,
							Dependencies: filtered,
						})
						l.Unlock()

						s.Done()
					}()
				}
			}
		}

		s.Wait()

		return nil
	})

	return binaries
}

func trimQuotes(imp string) string {
	return strings.Replace(imp, "\"", "", -1)
}
