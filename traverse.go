package main

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)


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

	dirKeys := make([]string, len(dirs))
	for dirKey, _ := range dirs {
		dirKeys = append(dirKeys, dirKey)
	}

	sort.Strings(dirKeys)

	for _, dirKey := range dirKeys {
		directory := dirs[dirKey]
		if directory == nil {
			continue
		}

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

						slog.Debug(
							"got resource",
							slog.String("directory", dirKey),
							slog.String("name", resourceName),
							slog.String("type", resourceType),
							slog.String("field_name", nameField),
							slog.String("file", fullPath),
						)
					}
				}

				if len(block.Labels) == 1 && block.Type == "module" {
					moduleResourceName := block.Labels[0]
					sourceField, versionField, _ := getSourceVersionFields(block)
					directory.Modules[moduleResourceName+":"+sourceField+"@"+versionField] = nil

					slog.Debug(
						"got module reference",
						slog.String("directory", dirKey),
						slog.String("name", moduleResourceName),
						slog.String("source", sourceField+"@"+versionField),
						slog.String("file", fullPath),
					)
				}
			}
		}
	}
}

func processModulesInDirs(dirs map[string]*Directory, dirsModules map[string]*Directory) {
	dirKeys := make([]string, len(dirs))
	for dirKey, _ := range dirs {
		dirKeys = append(dirKeys, dirKey)
	}

	sort.Strings(dirKeys)

	for _, dirKey := range dirKeys {
		directory := dirs[dirKey]
		if directory == nil {
			continue
		}

		if len(directory.Modules) == 0 {
			continue
		}

		moduleKeys := make([]string, len(directory.Modules))
		for moduleKey, _ := range directory.Modules {
			moduleKeys = append(moduleKeys, moduleKey)
		}

		sort.Strings(moduleKeys)

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
				slog.Error(
					"external module not found in allDirs",
					slog.String("module", modulePath),
				)
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
				slog.Error(
					"root not found for external module",
					slog.String("module", modulePath),
				)
				continue
			}

			directory.Modules[moduleKey] = rootExternalDirModule

			resourcesInside := make([]string, len(rootExternalDirModule.Resources))
			for _, resource := range rootExternalDirModule.Resources {
				resourcesInside = append(resourcesInside, resource.Type+"."+resource.Name)
			}
			resourcesInsideString := strings.Join(resourcesInside,",")

			slog.Debug(
				"assigned external module to directory modules",
				slog.String("dir", directory.FullPath),
				slog.String("module_key", moduleKey),
				slog.String("dir_module_module_name", rootExternalDirModule.ModuleName),
				slog.Int("dir_module_resources_num", len(rootExternalDirModule.Resources)),
				slog.Int("dir_module_modules_num", len(rootExternalDirModule.Modules)),
				slog.String("resources_inside", resourcesInsideString),
			)
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
