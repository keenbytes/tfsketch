package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

var (
	errWalkDir        = errors.New("error walking directory")
	errReadDir        = errors.New("error reading directory")
	errParseDirFiles  = errors.New("error parsing directory files")
	errLinkDirModules = errors.New("error linking directory modules")
	errParseHCLFile   = errors.New("error parsing HCL file")
)

func errWalkDirWithPath(path string) error {
	return fmt.Errorf("%w: %s", errWalkDir, path)
}

func errReadDirWithPath(path string) error {
	return fmt.Errorf("%w: %s", errReadDir, path)
}

func errParseDirFilesWithPath(path string) error {
	return fmt.Errorf("%w: %s", errParseDirFiles, path)
}

func errLinkDirModulesWithPath(path string) error {
	return fmt.Errorf("%w: %s", errLinkDirModules, path)
}

var (
	regexpDirPartToIgnore         = regexp.MustCompile(`^example[s]*$`)
	regexpDirPartIndicatingModule = regexp.MustCompile(`^modules$`)
)

const (
	dirTypeNormal = iota
	dirTypeModule
	dirTypeIgnore
)

const (
	tfExtension = ".tf"
)

// tfPath represents a path that contains terraform code.
type tfPath struct {
	// path is the full path.
	path string

	// relPath is a relative path - hence does not contain the parent/base.
	relPath string

	// tfPaths contains directories found in the path.
	tfPaths map[string]*tfPath

	// tfPathsModules contains map with names of paths that are modules.
	tfPathsModules map[string]struct{}

	// moduleSource is the source of the module (as in Override.Source) if the tfPath is a path to terraform module.
	moduleSource string

	// resources contains tf resources found in the code
	resources map[string]*resource

	// modules contains tf modules found in the code
	modules map[string]*module
}

func (t *tfPath) walkDir() error {
	errwd := filepath.WalkDir(t.path, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: %w", errWalkDir, err)
		}

		// check if the directory is not a module or if it's not meant to be ignored
		var dirType int
		relPath, _ := filepath.Rel(t.path, path)
		pathParts := strings.Split(filepath.ToSlash(relPath), "/")
		for _, part := range pathParts {
			if regexpDirPartToIgnore.MatchString(part) {
				dirType = dirTypeIgnore

				// skip the directory or file
				if dirEntry.IsDir() {
					return fs.SkipDir
				} else {
					return nil
				}
			}

			if regexpDirPartIndicatingModule.MatchString(part) {
				dirType = dirTypeModule

				break
			}
		}

		if dirEntry.IsDir() || !strings.HasSuffix(dirEntry.Name(), tfExtension) {
			return nil
		}

		dir := filepath.Dir(path)

		relDir, _ := filepath.Rel(t.path, dir)

		_, subTfPathExists := t.tfPaths[dir]
		if subTfPathExists {
			return nil
		}

		if relDir != "" {
			t.tfPaths[dir] = &tfPath{
				path:         dir,
				relPath:      relDir,
				resources:    map[string]*resource{},
				modules:      map[string]*module{},
				moduleSource: t.moduleSource,
			}

			if dirType == dirTypeModule {
				t.tfPathsModules[dir] = struct{}{}
			}
		}

		return nil
	})

	if errwd != nil {
		return fmt.Errorf("%w: %w", errWalkDirWithPath(t.path), errwd)
	}

	return nil
}

func (t *tfPath) parse(resourceType string) error {
	parser := hclparse.NewParser()

	err := t.parseFiles(parser, t, resourceType)
	if err != nil {
		return fmt.Errorf("%w: %w", errParseDirFilesWithPath(t.path), err)
	}

	pathsSorted := t.tfPathsSorted()
	for _, pathKey := range pathsSorted {
		subTfPath := t.tfPaths[pathKey]
		if subTfPath == nil {
			continue
		}

		slog.Debug(
			"opening tf path to parse files",
			slog.String("path", pathKey),
		)

		err := t.parseFiles(parser, subTfPath, resourceType)
		if err != nil {
			return fmt.Errorf("%w: %w", errParseDirFilesWithPath(subTfPath.path), err)
		}
	}

	return nil
}

func (t *tfPath) parseFiles(parser *hclparse.Parser, tfPath *tfPath, resourceType string) error {
	files, err := os.ReadDir(tfPath.path)
	if err != nil {
		return fmt.Errorf("%w: %w", errReadDirWithPath(t.path), err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), tfExtension) {
			continue
		}

		fileFullPath := filepath.Join(tfPath.path, file.Name())
		slog.Debug(
			"opening tf file to parse",
			slog.String("file", fileFullPath),
		)

		err := t.parseFile(parser, fileFullPath, tfPath, resourceType, file.Name())
		if err != nil {
			slog.Debug(
				"hcl parse error",
				slog.String("file", fileFullPath),
				slog.String("error", err.Error()),
			)

			// skip an invalid tf file
			continue
		}
	}

	return nil
}

func (t *tfPath) parseFile(parser *hclparse.Parser, filePath string, subTfPath *tfPath, resourceType string, fileName string) error {
	hclFile, diags := parser.ParseHCLFile(filePath)
	if diags.HasErrors() {

		return fmt.Errorf("%w: %s", errParseHCLFile, diags.Error())
	}

	content, _, _ := hclFile.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "resource", LabelNames: []string{"kind", "name"}},
			{Type: "module", LabelNames: []string{"name"}},
		},
	})

	for _, block := range content.Blocks {
		if len(block.Labels) == 2 && block.Type == "resource" {
			resource, _ := t.parseHCLBlockResource(block, resourceType)
			if resource == nil {
				continue
			}

			resource.fileName = fileName
			resource.filePath = filePath
			subTfPath.resources[resource.name] = resource

			slog.Debug(
				"got resource",
				slog.String("directory", subTfPath.path),
				slog.String("file", filePath),
				slog.String("type", resourceType),
				slog.String("name", resource.name),
				slog.String("field_name", resource.fieldName),
				slog.String("field_for_each", resource.fieldForEach),
			)
		}

		if len(block.Labels) == 1 && block.Type == "module" {
			module, _ := t.parseHCLBlockModule(block)
			if module == nil {
				continue
			}

			if module.fieldSource == "../" {
				slog.Debug(
					"ignoring module because of source",
					slog.String("directory", subTfPath.path),
					slog.String("file", filePath),
					slog.String("field_source", "../"),
					slog.String("type", "module"),
					slog.String("name", module.name),
				)

				continue
			}

			module.fileName = fileName
			module.filePath = filePath

			moduleKey := module.name + ":" + module.fieldSource + "@" + module.fieldVersion
			subTfPath.modules[moduleKey] = module

			slog.Debug(
				"got module reference",
				slog.String("directory", subTfPath.path),
				slog.String("file", filePath),
				slog.String("name", module.name),
				slog.String("source", moduleKey),
				slog.String("field_for_each", module.fieldForEach),
			)
		}
	}

	return nil
}

func (t *tfPath) parseHCLBlockResource(block *hcl.Block, resourceTypeToMatch string) (*resource, error) {
	resourceType := block.Labels[0]

	if resourceType != resourceTypeToMatch {
		return nil, nil
	}

	resourceName := block.Labels[1]

	resourceInstance := &resource{
		typ:  resourceType,
		name: resourceName,
	}

	nameField, _ := getNameFromHCLBlock(block)
	resourceInstance.fieldName = nameField

	forEachField, _ := getForEachFromHCLBlock(block)
	resourceInstance.fieldForEach = forEachField

	return resourceInstance, nil
}

func (t *tfPath) parseHCLBlockModule(block *hcl.Block) (*module, error) {
	moduleName := block.Labels[0]

	moduleInstance := &module{
		name: moduleName,
	}

	sourceField, versionField, _ := getSourceFromHCLBlock(block)
	moduleInstance.fieldSource = sourceField
	moduleInstance.fieldVersion = versionField

	forEachField, _ := getForEachFromHCLBlock(block)
	moduleInstance.fieldForEach = forEachField

	return moduleInstance, nil
}

func (t *tfPath) linkModulesInSubdirectories() error {
	err := t.linkModules(t)
	if err != nil {
		return fmt.Errorf("%w: %w", errLinkDirModulesWithPath(t.path), err)
	}

	pathsSorted := t.tfPathsSorted()
	for _, pathKey := range pathsSorted {
		subTfPath := t.tfPaths[pathKey]
		if subTfPath == nil {
			continue
		}

		_, isModuleDir := t.tfPathsModules[pathKey]
		if isModuleDir {
			continue
		}

		err := t.linkModules(subTfPath)
		if err != nil {
			return fmt.Errorf("%w: %w", errLinkDirModulesWithPath(subTfPath.path), err)
		}
	}

	return nil
}

func (t *tfPath) linkModules(tfPath *tfPath) error {
	if len(tfPath.modules) == 0 {
		return nil
	}

	modulesSorted := tfPath.modulesSorted()
	for _, moduleSource := range modulesSorted {
		moduleSourceArray := strings.Split(moduleSource, ":")
		if len(moduleSourceArray) != 2 {
			continue
		}

		// local modules are './modules...@' because there is no version number
		modulePath := moduleSourceArray[1]
		if !strings.HasPrefix(modulePath, "./modules") {
			continue
		}

		modulePathTrimmed := strings.Replace(modulePath, "./", "", 1)
		modulePathTrimmed = strings.Replace(modulePathTrimmed, "@", "", 1)

		// search for the local module in existing tfPaths
		moduleKeyToSearch := filepath.Join(tfPath.path, modulePathTrimmed)
		_, ok := t.tfPathsModules[moduleKeyToSearch]
		if !ok {
			continue
		}

		// assign tfpath (with resources) to the module
		moduleTfPath, ok := t.tfPaths[moduleKeyToSearch]
		if !ok {
			continue
		}
		tfPath.modules[moduleSource].tfPath = moduleTfPath
		slog.Debug(
			"got module link to local module",
			slog.String("module_key", moduleKeyToSearch),
			slog.String("module_path", moduleTfPath.path),
			slog.String("path", tfPath.path),
			slog.String("path_module_name", moduleSourceArray[0]),
			slog.String("path_module_path", modulePath),
			slog.Int("resource_num", len(moduleTfPath.resources)),
		)
	}

	return nil
}

func (t *tfPath) tfPathsSorted() []string {
	pathsSorted := make([]string, len(t.tfPaths))
	for pathKey, _ := range t.tfPaths {
		pathsSorted = append(pathsSorted, pathKey)
	}
	sort.Strings(pathsSorted)

	return pathsSorted
}

func (t *tfPath) modulesSorted() []string {
	modulesSorted := make([]string, len(t.modules))
	for moduleKey, _ := range t.modules {
		modulesSorted = append(modulesSorted, moduleKey)
	}
	sort.Strings(modulesSorted)

	return modulesSorted
}

func (t *tfPath) resourcesSorted() []string {
	resourcesSorted := make([]string, len(t.modules))
	for resourceKey, _ := range t.resources {
		resourcesSorted = append(resourcesSorted, resourceKey)
	}
	sort.Strings(resourcesSorted)

	return resourcesSorted
}
