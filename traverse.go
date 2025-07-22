package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

var (
	errTfDirWalk = errors.New("error walking tf dir")
)

type Directory struct {
	FullPath string
	DisplayPath string
	Resources map[string]*Resource
}

type Resource struct {
	Type string
	Name string
	FieldName string
	TfFileName string
}

func traverseTerraformDirectory(root string, resourceType string) error {
	dirs := make(map[string]*Directory)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: %w", errTfDirWalk, err)
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".tf") {
			dir := filepath.Dir(path)
			dirWithoutRoot := strings.Replace(dir, root, "", 1)
			dirs[dir] = &Directory{
				FullPath: dir,
				DisplayPath: dirWithoutRoot,
				Resources: map[string]*Resource{},
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error walking the path: %v\n", err)
		os.Exit(1)
	}

	processDirs(dirs, resourceType)

	genMermaid(dirs)

	return nil
}

func processDirs(dirs map[string]*Directory, resourceTypeToMatch string) {
	parser := hclparse.NewParser()

	for _, directory := range dirs {
		files, _ := os.ReadDir(directory.FullPath)
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".tf") {
				continue
			}

			fullPath := filepath.Join(directory.FullPath, file.Name())

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
				if len(block.Labels) != 2 || block.Type != "resource"{
					continue
				}
				resourceType := block.Labels[0]
				resourceName := block.Labels[1]
				if resourceType == resourceTypeToMatch {
					nameField, _ := getResourceNameField(block)
					// todo: handle error

					directory.Resources[resourceName] = &Resource{
						Type: resourceType,
						Name: resourceName,
						FieldName: nameField,
						TfFileName: file.Name(),
					}
				}
			}
		}
	}
}

var (
	TfResourceWithName = &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "name",
				Required: false,
			},
			{
				Name: "id",
				Required: false,
			},
		},
	}
)

func getResourceNameField(block *hcl.Block) (string, error) {
	name := block.Labels[0]

	bodyContent, _, diags := block.Body.PartialContent(TfResourceWithName)
	if diags.HasErrors() {
		return "", fmt.Errorf("error parsing block %s.%s: %s", block.Type, name, diags.Error())
	}

	nameField := ""
	
	for attrName, attr := range bodyContent.Attributes {
		if attrName == "name" {
			if expr, ok := attr.Expr.(*hclsyntax.TemplateExpr); ok {
				// This is an interpolated string like "${var.prefix}-example-role"
				srcRange := expr.SrcRange
				source, err := os.ReadFile(srcRange.Filename)
				if err == nil {
					raw := string(source[srcRange.Start.Byte:srcRange.End.Byte])
					nameField = raw
				}
			} else {
				// Handle plain expressions (e.g., just "foo")
				nameField = attr.Expr.Range().String()
			}
		}
	}

	return nameField, nil
}

func genMermaid(dirs map[string]*Directory) {
	mermaidDiagram := &strings.Builder{}

	mermaidDiagram.WriteString(`---
config:
  theme: redux
---
flowchart LR
  classDef tf-path fill:#c87de8
  classDef tf-resource-name stroke:#e7b6fc,color:#c87de8
  classDef aws-resource fill:#ffb83d
`)

	for _, dir := range dirs {
		elementPathName := strings.ReplaceAll(dir.DisplayPath, "/", "_")
		elementPathName = clearString(elementPathName)

		for _, resource := range dir.Resources{
			elementResourceName := elementPathName + "_" + clearString(resource.Name)
			elementResourceFieldName := elementResourceName + "_FieldName"

			_, _ = mermaidDiagram.WriteString(fmt.Sprintf("  %s[\"%s\"]:::tf-path --> %s[\"%s\"]:::tf-resource-name --> %s[%s]:::aws-resource\n", elementPathName, dir.DisplayPath, elementResourceName, resource.Name, elementResourceFieldName, resource.FieldName))
		}
	}

	fmt.Fprint(os.Stdout, mermaidDiagram.String())
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)

func clearString(str string) string {
    return nonAlphanumericRegex.ReplaceAllString(str, "")
}
