package whip

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	fixtureBase string
	projectRoot string
)

func FixturePath(p string) string {
	return filepath.Join(FixtureBase(), p)
}

func FixtureBase() string {
	if fixtureBase != "" {
		return fixtureBase
	}
	base := GetProjectRoot()
	fixtureBase, _ = filepath.EvalSymlinks(filepath.Join(base, "fixture"))
	return fixtureBase
}

func GetProjectRoot() string {
	if projectRoot != "" {
		return projectRoot
	}
	_, filename, _, _ := runtime.Caller(0)
	projectRoot, _ = filepath.EvalSymlinks(filepath.Join(path.Dir(filename), ".."))
	return projectRoot
}

func IsTest() bool {
	return strings.HasSuffix(os.Args[0], ".test")
}

func GetSelfDir() string {
	p, e := os.Executable()
	if e != nil {
		panic(e)
	}
	return filepath.Dir(p)
}
