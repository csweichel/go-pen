package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/evanw/esbuild/pkg/api"
)

func main() {
	root, err := repoRoot()
	if err != nil {
		fail(err.Error())
	}

	entry := filepath.Join(root, "pkg", "live", "ui", "main.tsx")
	outfile := filepath.Join(root, "pkg", "live", "app.js")

	result := api.Build(api.BuildOptions{
		EntryPoints: []string{entry},
		Bundle:      true,
		Format:      api.FormatIIFE,
		Platform:    api.PlatformBrowser,
		Target:      api.ES2020,
		Outfile:     outfile,
		Write:       true,
		Charset:     api.CharsetUTF8,
		Loader: map[string]api.Loader{
			".ts":  api.LoaderTS,
			".tsx": api.LoaderTSX,
		},
		JSXFactory:  "React.createElement",
		JSXFragment: "React.Fragment",
		LogLevel:    api.LogLevelInfo,
	})

	if len(result.Errors) > 0 {
		for _, msg := range result.Errors {
			fmt.Fprintln(os.Stderr, msg.Text)
		}
		os.Exit(1)
	}
}

func repoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("cannot find go.mod above %s", cwd)
		}
		dir = parent
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
