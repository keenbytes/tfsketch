package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
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
	ModuleName string
}

type Resource struct {
	Type       string
	Name       string
	FieldName  string
	TfFileName string
}

type DirContainer struct {
	Root string
	Dirs map[string]*Directory
	DirsModules map[string]*Directory
}

func traverseTerraformDirectory(root string, externalModuleName string, resourceType string) error {
	dirs, dirsModules, err := walkDirAndReturnsDirectories(root, externalModuleName)
	if err != nil {
		return fmt.Errorf("error walking dir: %w", err)
	}

	dirContainer := &DirContainer{
		Root: root,
		Dirs: dirs,
		DirsModules: dirsModules,
	}

	allDirs[externalModuleName] = dirContainer
	// put modules in the external modules in external modules as well
	/*if externalModuleName != "." {
		for moduleKey, _ := range dirsModules {
			relativePathToTheModule, _ := filepath.Rel(root, moduleKey)
			if relativePathToTheModule != "" {
				externalModuleInModuleName := externalModuleName + "|" + relativePathToTheModule
				moduleDirContainer := &DirContainer{
					Root: moduleKey,
					Dirs: dirsModules,
					DirsModules: map[string]*Directory{},
				}
				allDirs[externalModuleInModuleName] = moduleDirContainer
			}
		}
	}*/

	processDirs(dirs, resourceType)
	processDirs(dirsModules, resourceType)

	processModulesInDirs(dirs, dirsModules)

	return nil
}

func walkDirAndReturnsDirectories(root string, externalModuleName string) (map[string]*Directory, map[string]*Directory, error) {
	dirs := make(map[string]*Directory)
	dirsModules := make(map[string]*Directory)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: %w", errTfDirWalk, err)
		}

		isModulesDir := false

		// check for paths that have a 'modules' part in it
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
			if dirWithoutRoot == "" {
				dirWithoutRoot = "root"
			}

			if isModulesDir {
				_, dirAlreadyExists := dirsModules[dir]
				if !dirAlreadyExists {
					dirsModules[dir] = &Directory{
						FullPath:    dir,
						DisplayPath: dirWithoutRoot,
						Resources:   map[string]*Resource{},
						Modules: map[string]*Directory{},
						ModuleName: externalModuleName,
					}
				}
			} else {
				_, dirAlreadyExists := dirs[dir]
				if !dirAlreadyExists {
					dirs[dir] = &Directory{
						FullPath:    dir,
						DisplayPath: dirWithoutRoot,
						Resources:   map[string]*Resource{},
						Modules: map[string]*Directory{},
						ModuleName: externalModuleName,
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error walking the path: %w", err)
	}

	return dirs, dirsModules, nil
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

			slog.Debug(
				"opening tf file to parse in processDirs",
				slog.String("path", fullPath),
			)

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
					sourceField, versionField, _ := getSourceVersionFields(block)
					directory.Modules[moduleResourceName+":"+sourceField+"@"+versionField] = nil
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
			{
				Name:     "name_prefix",
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
			{
				Name:     "version",
				Required: false,
			},
		},
	}
)

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

			// ./modules means that the module in the same dir
			if strings.HasPrefix(modulePath, "./modules") {
				// local modules have './modules...@' because there is no version number
				modulePathTrimmed := strings.Replace(modulePath, "./", "", 1)
				modulePathTrimmed = strings.Replace(modulePathTrimmed, "@", "", 1)

				// search for module in existing "modules" dirs
				dirModule, ok := dirsModules[filepath.Join(directory.FullPath, modulePathTrimmed)]
				if !ok {
					continue
				}
				
				// we got a reference to a module that is local, meaning in a "modules" subdir
				// let's assign the Directory object so that we can later iterate over its resources
				directory.Modules[moduleKey] = dirModule

				continue
			}

			if directory.ModuleName != "." {
				modulePath = directory.ModuleName + "|" + strings.Replace(modulePath, "@", "", 1)
			}

			// everything else means it is an external module which we have to look in the overrides
			// allDirs is global but let's nevermind that for this PoC
			externalDirModule, ok := allDirs[modulePath]
			if !ok {
				/*if strings.HasPrefix(modulePath, ".") {
					modulePathTrimmed := strings.Replace(modulePath, "@", "", 1)


					moduleCleanPath := filepath.Clean(filepath.Join(directory.FullPath, modulePathTrimmed))
					for allDirModuleName, allDirItem := range allDirs {
						if moduleCleanPath == allDirItem.Root {
							externalDirModule = allDirs[allDirModuleName]
						}
					}
				}*/
				log.Printf("-------------> EXTERNAL MODULE NOT FOUND in allDirs: %s\n", modulePath)
				continue
			}

			// todo: this logic is wrong but let's leave it for now if it works
			// basically key in Dirs is the path because that how the traverse function works
			firstExternalDirModuleKey := ""
			for key, _ := range externalDirModule.Dirs {
				firstExternalDirModuleKey = key
			}

			rootExternalDirModule, ok := externalDirModule.Dirs[firstExternalDirModuleKey]
			if !ok {
				log.Printf("-------------> ROOT NOT FOUND IN EXTERNAL MODULE: %s\n", modulePath)
				continue
			}
			directory.Modules[moduleKey] = rootExternalDirModule
		}
	}
}

func getResourceNameField(block *hcl.Block) (string, error) {
	name := block.Labels[0]

	bodyContent, _, diags := block.Body.PartialContent(TfResourceWithName)
	if diags.HasErrors() {
		return "", fmt.Errorf("error parsing block %s.%s: %s", block.Type, name, diags.Error())
	}

	nameField := ""

	attrToLook := ""

	foundName := false
	foundNamePrefix := false
	foundId := false
	for attrName, _ := range bodyContent.Attributes {
		if attrName == "name" {
			foundName = true
		}
		if attrName == "name_prefix" {
			foundNamePrefix = true
		}
		if attrName == "id" {
			foundId = true
		}
	}

	if foundName {
		attrToLook = "name"
	}
	if !foundName && foundNamePrefix {
		attrToLook = "name_prefix"
	}
	if !foundName && !foundNamePrefix && foundId {
		attrToLook = "id"
	}

	if attrToLook == "" {
		return "no-name-attr", nil
	}

	for attrName, attr := range bodyContent.Attributes {
		if attrName != attrToLook {
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

func getSourceVersionFields(block *hcl.Block) (string, string, error) {
	name := block.Labels[0]

	bodyContent, _, diags := block.Body.PartialContent(TfModule)
	if diags.HasErrors() {
		return "", "", fmt.Errorf("error parsing block %s.%s: %s", block.Type, name, diags.Error())
	}

	fieldValues := make(map[string]string, 2)

	for attrName, attr := range bodyContent.Attributes {
		if attrName != "source" && attrName != "version" {
			continue
		}

		value, _ := attr.Expr.Value(nil)
		if value.Type() == cty.String {
			fieldValues[attrName] = value.AsString()
		}
	}

	return fieldValues["source"], fieldValues["version"], nil
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
	classDef tf-resource-name-from-internal-module fill:#e7b6fc
	classDef tf-resource-name-from-external-module fill:#7da8e8
  classDef tf-resource-field-name fill:#eb91c7
`)

	for _, dir := range dirs {
		elementPathName := strings.ReplaceAll(dir.DisplayPath, "/", "_")
		elementPathName = clearString(elementPathName)

		for _, resource := range dir.Resources {
			elementResourceName := elementPathName + "_" + clearString(resource.Name)
			elementResourceFieldName := elementResourceName + "_FieldName"

			_, _ = mermaidDiagram.WriteString(
				fmt.Sprintf(
					"  %s[\"%s\"]:::tf-path --> %s[\"%s\"]:::tf-resource-name --> %s[\"%s\"]:::tf-resource-field-name\n",
					elementPathName,
					dir.DisplayPath,
					elementResourceName,
					resource.Name,
					elementResourceFieldName,
					resource.FieldName,
				),
			)
		}

		writeModulesDiagramCode(mermaidDiagram, dir.Modules, elementPathName, dir.DisplayPath)
	}

	fmt.Fprint(os.Stdout, mermaidDiagram.String())
}

func writeModulesDiagramCode(mermaidDiagram *strings.Builder, dirModules map[string]*Directory, elementPathName string, dirDisplayPath string) {
	for moduleKey, dirModule := range dirModules {
		if dirModule == nil {
			continue
		}

		modElementPathName := strings.ReplaceAll(dirModule.DisplayPath, "/", "_")
		modElementPathName = "_mod_"+clearString(modElementPathName)

		modKeyValues := strings.Split(moduleKey, ":")
		modResourceName := modKeyValues[0]
		modPath := modKeyValues[1]

		// looping through module resources
		for _, resource := range dirModule.Resources {
			elemResourceName := elementPathName + modElementPathName + "_" + clearString(modResourceName) + "_" + clearString(resource.Name)
			elemResourceFieldName := elemResourceName + "_FieldName"

			elementClassDef := "tf-resource-name-from-external-module"
			if strings.HasPrefix(modPath, "./modules") {
				elementClassDef = "tf-resource-name-from-internal-module"
			}

			_, _ = mermaidDiagram.WriteString(
				fmt.Sprintf(
					"  %s[\"%s\"]:::tf-path --> %s[\"%s\"]:::%s --> %s[\"%s\"]:::tf-resource-field-name\n",
					elementPathName,
					dirDisplayPath,
					elemResourceName,
					"mod." + modResourceName + "." + resource.Name,
					elementClassDef,
					elemResourceFieldName,
					resource.FieldName,
				),
			)
		}

		if len(dirModule.Modules) == 0 {
			continue
		}

		writeModulesDiagramCode(mermaidDiagram, dirModule.Modules, elementPathName, dirDisplayPath)
	}
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)

func clearString(str string) string {
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}
