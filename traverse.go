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
	"github.com/zclconf/go-cty/cty"
)

var (
	errTfDirWalk = errors.New("error walking tf dir")
)

type Directory struct {
	FullPath    string
	DisplayPath string
	Resources   map[string]*Resource
	Modules map[string]*Directory
}

type Resource struct {
	Type       string
	Name       string
	FieldName  string
	TfFileName string
}

func traverseTerraformDirectory(root string, resourceType string) error {
	dirs := make(map[string]*Directory)
	dirsModules := make(map[string]*Directory)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: %w", errTfDirWalk, err)
		}

		isModulesDir := false

		// Skip directories that contain "modules" in their path
		relPath, _ := filepath.Rel(root, path)
		pathParts := strings.Split(filepath.ToSlash(relPath), "/")
		for _, part := range pathParts {
			if part == "modules" {
				isModulesDir = true
				break
			}
		}

		if !d.IsDir() && strings.HasSuffix(d.Name(), ".tf") {
			dir := filepath.Dir(path)
			dirWithoutRoot := strings.Replace(dir, root, "", 1)

			if isModulesDir {
				dirsModules[dir] = &Directory{
					FullPath:    dir,
					DisplayPath: dirWithoutRoot,
					Resources:   map[string]*Resource{},
				}
			} else {
				dirs[dir] = &Directory{
					FullPath:    dir,
					DisplayPath: dirWithoutRoot,
					Resources:   map[string]*Resource{},
					Modules: map[string]*Directory{},
				}
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error walking the path: %v\n", err)
		os.Exit(1)
	}

	processDirs(dirs, resourceType)
	processDirs(dirsModules, resourceType)
	processModulesInDirs(dirs, dirsModules)

	genMermaid(dirs)

	return nil
}

func processModulesInDirs(dirs map[string]*Directory, dirsModules map[string]*Directory) {
	for _, directory := range dirs {
		if len(directory.Modules) == 0 {
			continue
		}

		moduleKeys := make([]string, len(directory.Modules))
		for moduleKey, _ := range directory.Modules {
			moduleKeys = append(moduleKeys, moduleKey)
		}

		for _, moduleKey := range moduleKeys {
			// key = ResourceName:Path
			moduleKeyValues := strings.Split(moduleKey, ":")
			if len(moduleKeyValues) != 2 {
				continue
			}

			modulePath := moduleKeyValues[1]

			if !strings.HasPrefix(modulePath, "./modules") {
				continue
			}

			modulePathTrimmed := strings.Replace(modulePath, "./", "", 1)

			// search for module in existing "modules" dirs
			dirModule, ok := dirsModules[filepath.Join(directory.FullPath, modulePathTrimmed)]
			if !ok {
				continue
			}
			
			// we got a reference to a module that is local, meaning in a "modules" subdir
			// let's assign the Directory object so that we can later iterate over its resources
			directory.Modules[moduleKey] = dirModule
		}
	}
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
					{Type: "module", LabelNames: []string{"name"}},
				},
			})

			for _, block := range content.Blocks {
				if len(block.Labels) == 2 && block.Type == "resource" {
					resourceType := block.Labels[0]
					resourceName := block.Labels[1]
					if resourceType == resourceTypeToMatch {
						nameField, _ := getResourceNameField(block)
						// todo: handle error

						directory.Resources[resourceName] = &Resource{
							Type:       resourceType,
							Name:       resourceName,
							FieldName:  nameField,
							TfFileName: file.Name(),
						}
					}
				}

				if len(block.Labels) == 1 && block.Type == "module" {
					moduleResourceName := block.Labels[0]
					sourceField, _ := getSourceField(block)
					directory.Modules[moduleResourceName+":"+sourceField] = nil
				}
			}
		}
	}
}

var (
	TfResourceWithName = &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "name",
				Required: false,
			},
			{
				Name:     "id",
				Required: false,
			},
		},
	}
	TfModule = &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "source",
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
		if attrName != "name" {
			continue
		}

		expr, ok := attr.Expr.(*hclsyntax.TemplateExpr)
		if ok {
			srcRange := expr.SrcRange
			source, err := os.ReadFile(srcRange.Filename)
			if err == nil {
				raw := string(source[srcRange.Start.Byte:srcRange.End.Byte])
				nameField = raw
				nameField = strings.TrimLeft(nameField, "\"")
				nameField = strings.TrimRight(nameField, "\"")
			}

			continue
		}

		scopeTraversalExpr, ok := attr.Expr.(*hclsyntax.ScopeTraversalExpr)
		if ok {
			srcRange := scopeTraversalExpr.SrcRange
			source, err := os.ReadFile(srcRange.Filename)
			if err == nil {
				raw := string(source[srcRange.Start.Byte:srcRange.End.Byte])
				nameField = raw
				nameField = strings.TrimLeft(nameField, "\"")
				nameField = strings.TrimRight(nameField, "\"")
			}
		}
	}

	return nameField, nil
}

func getSourceField(block *hcl.Block) (string, error) {
	name := block.Labels[0]

	bodyContent, _, diags := block.Body.PartialContent(TfModule)
	if diags.HasErrors() {
		return "", fmt.Errorf("error parsing block %s.%s: %s", block.Type, name, diags.Error())
	}

	source := ""

	for attrName, attr := range bodyContent.Attributes {
		if attrName != "source" {
			continue
		}

		value, _ := attr.Expr.Value(nil)
		if value.Type() == cty.String {
			source = value.AsString()
		}
	}

	return source, nil
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

		for _, resource := range dir.Resources {
			elementResourceName := elementPathName + "_" + clearString(resource.Name)
			elementResourceFieldName := elementResourceName + "_FieldName"

			_, _ = mermaidDiagram.WriteString(
				fmt.Sprintf(
					"  %s[\"%s\"]:::tf-path --> %s[\"%s\"]:::tf-resource-name --> %s[\"%s\"]:::aws-resource\n",
					elementPathName,
					dir.DisplayPath,
					elementResourceName,
					resource.Name,
					elementResourceFieldName,
					resource.FieldName,
				),
			)
		}

		for moduleKey, dirModule := range dir.Modules {
			if dirModule == nil {
				continue
			}
			moduleElementPathName := strings.ReplaceAll(dirModule.DisplayPath, "/", "_")
			moduleElementPathName = clearString(elementPathName)

			moduleKeyValues := strings.Split(moduleKey, ":")
			moduleResourceName := moduleKeyValues[0]

			for _, resource := range dirModule.Resources {
				elementResourceName := elementPathName + "_mod_" + moduleElementPathName + "_" + clearString(moduleResourceName) + "_" + clearString(resource.Name)
				elementResourceFieldName := elementResourceName + "_FieldName"

				_, _ = mermaidDiagram.WriteString(
					fmt.Sprintf(
						"  %s[\"%s\"]:::tf-path --> %s[\"%s\"]:::tf-resource-name --> %s[\"%s\"]:::aws-resource\n",
						elementPathName,
						dir.DisplayPath,
						elementResourceName,
						"mod." + moduleResourceName + "." + resource.Name,
						elementResourceFieldName,
						resource.FieldName,
					),
				)
			}
		}
	}

	fmt.Fprint(os.Stdout, mermaidDiagram.String())
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)

func clearString(str string) string {
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}
