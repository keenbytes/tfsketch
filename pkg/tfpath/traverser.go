package tfpath

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

const (
	tfExtension             = ".tf"
	linkModulesMaxRecursion = 5
	labelNoFieldName        = "no-attr!"
	labelFieldNameEmpty     = "empty!"
)

// Traverser represents functionality for scanning a Terraform directory.
type Traverser struct {
	// RegexpIgnoreDir is a regular expression used to check if directory name should be ignored
	RegexpIgnoreDir *regexp.Regexp
	// RegexpModuleDir is a regular expression used to check if directory name is a module
	RegexpModuleDir *regexp.Regexp
	// Container contains all the modules that have been found so far
	Container *Container
	// RegexpIncludePath is used to include paths
	RegexpIncludePath *regexp.Regexp
	// RegexpExcludePath is used to exclude paths
	RegexpExcludePath *regexp.Regexp
	// RegexpResourceType is type of the resource to search, eg. ^aws_iam_role$
	RegexpResourceType *regexp.Regexp
	// RegexpResourceName is type of the resource to search, eg. ^this$
	RegexpResourceName *regexp.Regexp
	// DisplayAttributes contains is comma-separated resource attributes where the first found is used as the
	// chart’s display name
	DisplayAttributes []string
	// Parser is an HCL parser
	Parser *hclparse.Parser
	// HCLBodySchema contains a schema for parsing HCL. It defines what attributes should be extracted.
	HCLBodySchema *hcl.BodySchema
}

// NewTraverser returns new Traverser object.
func NewTraverser(
	container *Container,
	pathIncludeRegexp, pathExcludeRegexp, typeRegexp, nameRegexp, displayAttributes string,
) *Traverser {
	traverser := &Traverser{
		Parser:             hclparse.NewParser(),
		RegexpIgnoreDir:    regexp.MustCompile(`^(example[s]*|test[s]*|\..*)$`),
		RegexpModuleDir:    regexp.MustCompile(`^modules$`),
		Container:          container,
		RegexpIncludePath:  regexp.MustCompile(pathIncludeRegexp),
		RegexpExcludePath:  regexp.MustCompile(pathExcludeRegexp),
		RegexpResourceType: regexp.MustCompile(typeRegexp),
		RegexpResourceName: regexp.MustCompile(nameRegexp),
	}

	if displayAttributes != "" {
		traverser.DisplayAttributes = strings.Split(displayAttributes, ",")
	} else {
		traverser.DisplayAttributes = []string{"name", "id", "name_prefix"}
	}

	hclBodySchema := NewHCLBodySchema(traverser.DisplayAttributes)
	traverser.HCLBodySchema = hclBodySchema

	return traverser
}

// WalkPath walks a specified path for subdirectories.
func (t *Traverser) WalkPath(tfPath *TfPath, extractModules bool) error {
	newContainerPaths, err := t.walk(tfPath, extractModules)
	if err != nil {
		return fmt.Errorf("error walking terraform path %s: %s", tfPath.Path, err.Error())
	}

	if extractModules && len(newContainerPaths) > 0 {
		for _, newPath := range newContainerPaths {
			newTfPath := t.Container.Paths[newPath]

			_, err := t.walk(newTfPath, false)
			if err != nil {
				return fmt.Errorf(
					"error walking terraform path %s: %s",
					newTfPath.Path,
					err.Error(),
				)
			}
		}
	}

	return nil
}

// ParsePath scans a specified path, reads Terraform files and parses out modules and resources.
func (t *Traverser) ParsePath(tfPath *TfPath) error {
	err := t.parseFiles(tfPath)
	if err != nil {
		return fmt.Errorf("error parsing files in %s: %s", tfPath.Path, err.Error())
	}

	childrenNamesSorted := tfPath.ChildrenNamesSorted()
	for _, childrenName := range childrenNamesSorted {
		childTfPath := tfPath.Children[childrenName]
		if childTfPath == nil {
			continue
		}

		err := t.parseFiles(childTfPath)
		if err != nil {
			return fmt.Errorf("error parsing files in child %s: %s", tfPath.Path, err.Error())
		}
	}

	return nil
}

// LinkPath scans modules in a specified path and links them to already existing modules in the container,
// eg. external modules.
func (t *Traverser) LinkPath(tfPath *TfPath) error {
	t.link(tfPath, tfPath)

	for _, childTfPath := range tfPath.Children {
		t.link(tfPath, childTfPath)
	}

	return nil
}

//nolint:funlen
func (t *Traverser) walk(tfPath *TfPath, extractModules bool) ([]string, error) {
	rootPath := tfPath.Path

	newContainerPaths := []string{}

	errwd := filepath.WalkDir(
		rootPath,
		func(currentPath string, dirEntry fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("error walking directory %s: %s", currentPath, err.Error())
			}
			// ignore files
			if !dirEntry.IsDir() {
				return nil
			}

			currentRelPath, _ := filepath.Rel(rootPath, currentPath)
			currentParentDir := filepath.Dir(currentPath)
			currentParentDirName := filepath.Base(currentParentDir)
			currentDirName := filepath.Base(currentPath)

			// Include relative path based on a regular expression
			if !t.RegexpIncludePath.MatchString(currentRelPath) {
				slog.Debug(fmt.Sprintf("🚫 Skipped path: 📁%s [📁%s]", currentPath, currentRelPath))

				return fs.SkipDir
			}

			// Exclude relative path based on a regular expression
			if t.RegexpExcludePath.MatchString(currentRelPath) {
				slog.Debug(fmt.Sprintf("🚫 Skipped path: 📁%s [📁%s]", currentPath, currentRelPath))

				return fs.SkipDir
			}

			// if directory is called 'tests' or 'examples' then do not walk the dir
			if t.RegexpIgnoreDir.MatchString(currentDirName) {
				slog.Debug(fmt.Sprintf("🚫 Skipped path: 📁%s [📁%s]", currentPath, currentRelPath))

				return fs.SkipDir
			}

			// if subdirectory of 'modules' directory then add it to container and skip further directories
			if extractModules && t.RegexpModuleDir.MatchString(currentParentDirName) &&
				currentRelPath != "." {
				traverseNameSplit := strings.Split(tfPath.TraverseName, "@")

				newTraverseName := traverseNameSplit[0] + "//" + currentRelPath
				//nolint:mnd
				if len(traverseNameSplit) == 2 {
					newTraverseName += "@" + traverseNameSplit[1]
				}

				newTfPath := NewTfPath(currentPath, newTraverseName)
				newTfPath.RelPath = "."
				t.Container.AddPath(newTfPath.TraverseName, newTfPath)
				newContainerPaths = append(newContainerPaths, newTfPath.TraverseName)

				slog.Debug(
					fmt.Sprintf(
						"🚫 Skipped walking paths in: 📁%s [📁%s]",
						currentPath,
						currentRelPath,
					),
				)

				return fs.SkipDir
			}

			if currentRelPath == "." {
				return nil
			}

			_, childExists := tfPath.Children[currentRelPath]
			if childExists {
				slog.Debug(
					fmt.Sprintf(
						"🚫 Child terraform path already exist: 📁%s in 📁%s (📦%s)",
						currentRelPath,
						rootPath,
						tfPath.TraverseName,
					),
				)

				return nil
			}

			newTfPath := NewTfPath(currentPath, tfPath.TraverseName)
			newTfPath.RelPath = currentRelPath

			tfPath.Children[currentRelPath] = newTfPath
			slog.Debug(
				fmt.Sprintf(
					"🟣 Child terraform path added: 📁%s to 📁%s (📦%s)",
					currentRelPath,
					rootPath,
					tfPath.TraverseName,
				),
			)

			return nil
		},
	)
	if errwd != nil {
		return []string{}, fmt.Errorf(
			"error walking directories in terraform path %s (📦%s): %s",
			rootPath,
			tfPath.TraverseName,
			errwd.Error(),
		)
	}

	return newContainerPaths, nil
}

//nolint:gocognit,funlen
func (t *Traverser) link(rootTfParent *TfPath, childTfPath *TfPath) {
	for moduleName, module := range childTfPath.Modules {
		source := module.FieldSource
		version := module.FieldVersion

		if !strings.HasPrefix(source, ".") {
			containerPathKey := fmt.Sprintf("%s@%s", source, version)

			containerTfPath, exists := t.Container.Paths[containerPathKey]
			if !exists {
				slog.Info(
					fmt.Sprintf(
						"🚫 Skipped linking child terraform path 📁%s (📦%s) module %s due to source 📦%s not found in the container",
						childTfPath.Path,
						childTfPath.TraverseName,
						moduleName,
						containerPathKey,
					),
				)

				continue
			}

			module.TfPath = containerTfPath
			slog.Debug(
				fmt.Sprintf(
					"🟡 Linked module %s in path 📁%s (📦%s) to container path 📁%s (📦%s)",
					moduleName,
					childTfPath.RelPath,
					childTfPath.TraverseName,
					containerTfPath.Path,
					containerTfPath.TraverseName,
				),
			)

			continue
		}

		cleanPath := filepath.Clean(filepath.Join(childTfPath.Path, source))

		relPath, err := filepath.Rel(rootTfParent.Path, cleanPath)
		if err != nil {
			slog.Info(
				fmt.Sprintf(
					"🚫 Skipped linking child terraform path 📁%s (📦%s) to module %s due to problem with relative path: %s",
					childTfPath.Path,
					childTfPath.TraverseName,
					moduleName,
					err.Error(),
				),
			)

			continue
		}

		if relPath == "" || relPath == "." {
			continue
		}

		foundLink := false

		if strings.HasPrefix(relPath, "../") {
			// search in the container
			for _, containerTfPath := range t.Container.Paths {
				if containerTfPath.Path == cleanPath {
					module.TfPath = containerTfPath

					slog.Debug(
						fmt.Sprintf(
							"🟡 Linked module %s in path 📁%s (📦%s) to container path 📁%s (📦%s)",
							moduleName,
							childTfPath.RelPath,
							childTfPath.TraverseName,
							containerTfPath.Path,
							containerTfPath.TraverseName,
						),
					)

					foundLink = true

					break
				}
			}
		}

		if foundLink {
			continue
		}

		moduleTfPath, exists := rootTfParent.Children[relPath]
		if exists {
			if module.TfPath == nil {
				module.TfPath = moduleTfPath

				slog.Debug(
					fmt.Sprintf(
						"🟡 Linked module %s in path 📁%s (📦%s) to parent path 📁%s (📦%s)",
						moduleName,
						childTfPath.RelPath,
						childTfPath.TraverseName,
						moduleTfPath.Path,
						moduleTfPath.TraverseName,
					),
				)

				continue
			}
		}

		// if inside a module and module path does not contain '//modules' already then search in the container
		if rootTfParent.TraverseName != "." &&
			!strings.Contains(rootTfParent.TraverseName, "//modules/") &&
			strings.HasPrefix(source, "./modules/") {
			rootTfPathModule := strings.Split(rootTfParent.TraverseName, "@")
			rootTfPathSource := rootTfPathModule[0]
			rootTfPathVersion := rootTfPathModule[1]

			moduleToSearch := fmt.Sprintf("%s//%s@%s", rootTfPathSource, relPath, rootTfPathVersion)

			containerTfPath, exists := t.Container.Paths[moduleToSearch]
			if exists {
				module.TfPath = containerTfPath

				slog.Debug(
					fmt.Sprintf(
						"🟡 Linked module %s in path 📁%s (📦%s) to container path 📁%s (📦%s)",
						moduleName,
						childTfPath.RelPath,
						childTfPath.TraverseName,
						containerTfPath.Path,
						containerTfPath.TraverseName,
					),
				)

				continue
			}
		}

		slog.Info(
			fmt.Sprintf(
				"🚫 Skipped linking child terraform path 📁%s (📦%s) module %s due to relative path %s not found in its parent",
				childTfPath.Path,
				childTfPath.TraverseName,
				moduleName,
				relPath,
			),
		)
	}
}

func (t *Traverser) parseFiles(tfPath *TfPath) error {
	files, err := os.ReadDir(tfPath.Path)
	if err != nil {
		return fmt.Errorf("error reading directory %s: %s", tfPath.Path, err.Error())
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), tfExtension) {
			continue
		}

		fileFullPath := filepath.Join(tfPath.Path, file.Name())

		err := t.parseFile(tfPath, file.Name())
		if err != nil {
			slog.Error(fmt.Sprintf("❌ Error parsing file 📄%s: %s", fileFullPath, err.Error()))

			// skip an invalid tf file
			continue
		}
	}

	return nil
}

//nolint:funlen
func (t *Traverser) parseFile(tfPath *TfPath, fileName string) error {
	filePath := filepath.Join(tfPath.Path, fileName)

	hclFile, diags := t.Parser.ParseHCLFile(filePath)
	if diags.HasErrors() {
		return fmt.Errorf("error parsing hcl file: %s", diags.Error())
	}

	content, _, _ := hclFile.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "resource", LabelNames: []string{"kind", "name"}},
			{Type: "module", LabelNames: []string{"name"}},
		},
	})

	for _, block := range content.Blocks {
		if len(block.Labels) == 2 && block.Type == "resource" {
			resource := t.parseHCLBlockResource(block)
			if resource == nil {
				continue
			}

			resource.FileName = fileName
			resource.FilePath = filePath
			tfPath.Resources[resource.Type+"."+resource.Name] = resource

			slog.Info(
				fmt.Sprintf(
					"🟠 Found resource %s in file 📄%s (📦%s)",
					resource.Name,
					filePath,
					tfPath.TraverseName,
				),
			)

			if resource.FieldForEach != "" {
				slog.Info(
					fmt.Sprintf(
						"🟠 Found resource %s for_each is 🔄%s",
						resource.Name,
						resource.FieldForEach,
					),
				)
			}
		}

		if len(block.Labels) == 1 && block.Type == "module" {
			module := t.parseHCLBlockModule(block)
			if module == nil {
				continue
			}

			if module.FieldSource == "../" {
				slog.Debug(
					fmt.Sprintf(
						"💭 Ignoring module %s with source '../' in 📄%s",
						module.Name,
						filePath,
					),
				)

				continue
			}

			module.FileName = fileName
			module.FilePath = filePath

			tfPath.Modules[module.Name] = module

			slog.Info(
				fmt.Sprintf(
					"🔵 Found module %s in file 📄%s (📦%s)",
					module.Name,
					filePath,
					tfPath.TraverseName,
				),
			)

			if module.FieldForEach != "" {
				slog.Info(
					fmt.Sprintf(
						"🔵 Found module %s for_each is 🔄%s",
						module.Name,
						module.FieldForEach,
					),
				)
			}
		}
	}

	return nil
}

func (t *Traverser) parseHCLBlockResource(block *hcl.Block) *TfResource {
	resourceType := block.Labels[0]

	if !t.RegexpResourceType.MatchString(resourceType) {
		return nil
	}

	resourceName := block.Labels[1]

	if !t.RegexpResourceName.MatchString(resourceName) {
		return nil
	}

	resourceInstance := &TfResource{
		Type: resourceType,
		Name: resourceName,
	}

	nameField, _ := t.getNameFromHCLBlock(block)
	resourceInstance.FieldName = nameField

	forEachField, _ := t.getForEachFromHCLBlock(block)
	resourceInstance.FieldForEach = forEachField

	return resourceInstance
}

func (t *Traverser) parseHCLBlockModule(block *hcl.Block) *TfModule {
	moduleName := block.Labels[0]

	moduleInstance := &TfModule{
		Name: moduleName,
	}

	sourceField, versionField, _ := t.getSourceFromHCLBlock(block)
	moduleInstance.FieldSource = sourceField
	moduleInstance.FieldVersion = versionField

	forEachField, _ := t.getForEachFromHCLBlock(block)
	moduleInstance.FieldForEach = forEachField

	return moduleInstance
}

//nolint:funlen
func (t *Traverser) getNameFromHCLBlock(block *hcl.Block) (string, error) {
	name := block.Labels[0]

	bodyContent, _, diags := block.Body.PartialContent(t.HCLBodySchema)
	if diags.HasErrors() {
		return "", fmt.Errorf(
			"error getting partial content: %s.%s: %s",
			block.Type,
			name,
			diags.Error(),
		)
	}

	attrToGet := ""

	for attrName := range bodyContent.Attributes {
		for _, displayAttr := range t.DisplayAttributes {
			if attrName == displayAttr {
				attrToGet = attrName

				break
			}
		}

		if attrToGet != "" {
			break
		}
	}

	if attrToGet == "" {
		return labelNoFieldName, nil
	}

	nameField := ""

	for attrName, attr := range bodyContent.Attributes {
		if attrName != attrToGet {
			continue
		}

		var srcRange hcl.Range

		var found bool

		expr, ok := attr.Expr.(*hclsyntax.TemplateExpr)
		if ok {
			srcRange = expr.SrcRange
			found = true
		}

		if !found {
			scopeTraversalExpr, ok := attr.Expr.(*hclsyntax.ScopeTraversalExpr)
			if ok {
				srcRange = scopeTraversalExpr.SrcRange
				found = true
			}
		}

		if found {
			source, err := os.ReadFile(srcRange.Filename)
			if err == nil {
				raw := string(source[srcRange.Start.Byte:srcRange.End.Byte])
				nameField = raw
			}
		}
	}

	if nameField == "" {
		nameField = labelFieldNameEmpty
	}

	return nameField, nil
}

func (t *Traverser) getForEachFromHCLBlock(block *hcl.Block) (string, error) {
	name := block.Labels[0]

	bodyContent, _, diags := block.Body.PartialContent(t.HCLBodySchema)
	if diags.HasErrors() {
		return "", fmt.Errorf(
			"error getting partial content: %s.%s: %s",
			block.Type,
			name,
			diags.Error(),
		)
	}

	forEachField := ""

	for attrName, attr := range bodyContent.Attributes {
		if attrName != "for_each" {
			continue
		}

		var srcRange hcl.Range

		var found bool

		expr, ok := attr.Expr.(*hclsyntax.TupleConsExpr)
		if ok {
			srcRange = expr.SrcRange
			found = true
		}

		if !found {
			scopeTraversalExpr, ok := attr.Expr.(*hclsyntax.ScopeTraversalExpr)
			if ok {
				srcRange = scopeTraversalExpr.SrcRange
				found = true
			}
		}

		if found {
			source, err := os.ReadFile(srcRange.Filename)
			if err == nil {
				raw := string(source[srcRange.Start.Byte:srcRange.End.Byte])
				forEachField = raw
			}
		}
	}

	return forEachField, nil
}

func (t *Traverser) getSourceFromHCLBlock(block *hcl.Block) (string, string, error) {
	name := block.Labels[0]

	bodyContent, _, diags := block.Body.PartialContent(t.HCLBodySchema)
	if diags.HasErrors() {
		return "", "", fmt.Errorf(
			"error getting partial content: %s.%s: %s",
			block.Type,
			name,
			diags.Error(),
		)
	}

	//nolint:mnd
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
