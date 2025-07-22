package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

var (
	errTfDirWalk = errors.New("error walking tf dir")
)

func traverseTerraformDirectory(path string, resourceType string) error {
	dirs := make(map[string]struct{})

	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: %w", errTfDirWalk, err)
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".tf") {
			dir := filepath.Dir(path)
			dirs[dir] = struct{}{}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error walking the path: %v\n", err)
		os.Exit(1)
	}

	processDirs(dirs, resourceType)

	return nil
}

func processDirs(dirs map[string]struct{}, resourceType string) {
	parser := hclparse.NewParser()

	for dir := range dirs {
		files, _ := os.ReadDir(dir)
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".tf") {
				continue
			}
			fullPath := filepath.Join(dir, file.Name())
			hclFile, diags := parser.ParseHCLFile(fullPath)
			if diags.HasErrors() {
				continue // skip invalid files
			}

			content, _, _ := hclFile.Body.PartialContent(&hcl.BodySchema{
				Blocks: []hcl.BlockHeaderSchema{
					{Type: "resource", LabelNames: []string{"kind", "name"}},
				},
			})

			for _, block := range content.Blocks {
				if len(block.Labels) != 2 {
					continue
				}
				resourceType := block.Labels[0]
				resourceName := block.Labels[1]
				if resourceType == resourceType {
					fmt.Printf("%s: %s\n", fullPath, resourceName)
				}
			}
		}
	}
}
