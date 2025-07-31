package main

/*
import (
	"fmt"
	"html"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const mermaidConfig = `---
config:
  theme: redux
  flowchart:
    diagramPadding: 5
    padding: 5
    nodeSpacing: 10
    wrappingWidth: 700
---
flowchart LR
  classDef tf-path fill:#c87de8
  classDef tf-resource-name stroke:#e7b6fc,color:#c87de8,text-align:left
  classDef tf-int-mod fill:#e7b6fc,text-align:left
  classDef tf-ext-mod fill:#7da8e8,text-align:left
  classDef tf-resource-field-name fill:#eb91c7
`

const elementSeparator = "__"
const partSeparator = "_"

var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)

func genMermaid(tfPath *tfPath, resourceTypeToFind string, outputFile string) {
	mermaidDiagram := &strings.Builder{}
	resourceEdges := &strings.Builder{}

	mermaidDiagram.WriteString(mermaidConfig)

	// going through sub-paths only, not the root
	sortedPaths := tfPath.tfPathsSorted()
	for _, sortedPath := range sortedPaths {
		subTfPath := tfPath.tfPaths[sortedPath]
		if subTfPath == nil {
			continue
		}

		// don't render modules
		_, isModule := tfPath.tfPathsModules[sortedPath]
		if isModule {
			continue
		}

		slog.Info(
			"generating mermaid for path",
			slog.String("path", subTfPath.path),
		)

		// directory (tf path) element on the diagram
		elTfPathID := elID(subTfPath.relPath)
		elTfPathLabel := subTfPath.relPath
		elTfPath := elTfPath(elTfPathID, elTfPathLabel)

		// loop through resources
		sortedResources := subTfPath.resourcesSorted()
		for _, resourceKey := range sortedResources {
			resource := subTfPath.resources[resourceKey]
			if resource == nil {
				continue
			}

			slog.Info(
				"generating mermaid for resource",
				slog.String("path", subTfPath.path),
				slog.String("resource", resource.name),
			)

			partForEach := ""
			if resource.fieldForEach != "" {
				partForEach = "<br>*for_each = " + html.EscapeString(resource.fieldForEach) + "*"
			}

			elResourceID := elTfPathID + elementSeparator + elID(resource.name)
			elResourceLabel := resourceTypeToFind + "." + resource.name + partForEach
			elResource := elResource(elResourceID, elResourceLabel)

			elResourceNameID := elResourceID + partSeparator + "FieldName"
			elResourceNameLabel := html.EscapeString(resource.fieldName)
			elResourceName := elResourceName(elResourceNameID, elResourceNameLabel)

			_, _ = mermaidDiagram.WriteString(
				fmt.Sprintf(
					"  %s ---> %s --> %s\n",
					elTfPath,
					elResource,
					elResourceName,
				),
			)

			resourceEdges.WriteString(elResourceNameID + "\n")
		}

		genModules(mermaidDiagram, resourceEdges, subTfPath, elTfPathID, resourceTypeToFind, "", "")
	}

	//writeModulesDiagramCode(mermaidDiagram, dir.Modules, dir.ModulesForEach, elementTfPathID, elementTfPath, resourceTypeToFind, "", "", resourceEdges)

	err := os.WriteFile(filepath.Clean(outputFile), []byte(mermaidDiagram.String()), 0600)
	if err != nil {
		slog.Error(
			"error writing output file",
			slog.String("path", outputFile),
			slog.String("error", err.Error()),
		)
	}

	edgesFile := outputFile + ".edges.txt"
	err = os.WriteFile(filepath.Clean(edgesFile), []byte(resourceEdges.String()), 0600)
	if err != nil {
		slog.Error(
			"error writing file with edges",
			slog.String("path", edgesFile),
			slog.String("error", err.Error()),
		)
	}
}

func genModules(mermaidDiagram *strings.Builder, resourceEdges *strings.Builder, tfPath *tfPath, elTfPathID, resourceTypeToFind, elParentLabel, elParentID string) {
	sortedModules := tfPath.modulesSorted()

	slog.Info(
		"generating mermaid for modules",
		slog.String("path", tfPath.path),
	)

	for _, moduleSource := range sortedModules {
		module := tfPath.modules[moduleSource]
		if module == nil {
			continue
		}
		moduleSourceArray := strings.Split(moduleSource, ":")
		if len(moduleSourceArray) != 2 {
			continue
		}

		slog.Info(
			"generating mermaid for module",
			slog.String("path", tfPath.path),
			slog.String("module", moduleSource),
			slog.String("module_tfpath", fmt.Sprintf("%v", module.tfPath)),
		)

		moduleName := moduleSourceArray[0]
		modulePath := moduleSourceArray[1]

		// for_each field
		partForEach := ""
		if module.fieldForEach != "" {
			partForEach = "<br><i>for_each = " + html.EscapeString(module.fieldForEach) + "</i>"
		}

		// pass module name as a parent to the next module inside it
		elParentModuleLabel := ""
		if elParentLabel != "" {
			elParentModuleLabel = elParentLabel + "<br>-&gt;<br>"
		}

		// tidy up modulePath for to be displayed in the element's label
		modulePath = strings.TrimRight(modulePath, "@")
		modulePath = strings.Replace(modulePath, "@", `\@`, 1)

		elModuleLabel := elParentModuleLabel + "<b>module." + moduleName + "</b><br>" + modulePath + partForEach

		elModuleIDResourceNamePart := ""
		if elParentID != "" {
			elModuleIDResourceNamePart += elParentID + elementSeparator
		}
		elModuleIDResourceNamePart += elID(moduleName)
		elModuleID := elTfPathID + elementSeparator + elID(module.tfPath.relPath) + partSeparator + elModuleIDResourceNamePart + partSeparator + elID(module.name)

		elModule := ""
		if elParentID == "" && strings.HasPrefix(modulePath, "./modules") {
			elModule = elTfInternalModule(elModuleID, elModuleLabel)
		} else {
			elModule = elTfExternalModule(elModuleID, elModuleLabel)
		}

		// do not print a module that has no resources
		if len(module.tfPath.resources) > 0 {
			_, _ = mermaidDiagram.WriteString(
				fmt.Sprintf(
					"  %s --> %s\n",
					elTfPathID,
					elModule,
				),
			)
		}

		// looping through module resources
		for _, resource := range module.tfPath.resources {
			partForEach := ""
			if resource.fieldForEach != "" {
				partForEach = "<br><i>for_each = " + html.EscapeString(resource.fieldForEach) + "</i>"
			}

			elResourceID := elModuleID + elementSeparator + elID(resource.name)
			elResourceLabel := resourceTypeToFind + "." + resource.name + partForEach
			elResource := elResource(elResourceID, elResourceLabel)

			elResourceNameID := elResourceID + partSeparator + "FieldName"
			elResourceNameLabel := html.EscapeString(resource.fieldName)
			elResourceName := elResourceName(elResourceNameID, elResourceNameLabel)

			_, _ = mermaidDiagram.WriteString(
				fmt.Sprintf(
					"  %s --> %s --> %s\n",
					elModuleID,
					elResource,
					elResourceName,
				),
			)

			resourceEdges.WriteString(elResourceNameID + "\n")
		}

		if len(module.tfPath.modules) == 0 {
			continue
		}

		genModules(mermaidDiagram, resourceEdges, module.tfPath, elTfPathID, resourceTypeToFind, elModuleLabel, elModuleID)
	}
}

func clearString(str string) string {
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}

func elID(text string) string {
	text = strings.ReplaceAll(text, "/", "_")
	text = clearString(text)
	return text
}

func el(id, label, classDef string) string {
	return fmt.Sprintf("%s[\"%s\"]:::%s", id, label, classDef)
}

func elTfPath(id, label string) string {
	return el(id, label, "tf-path")
}

func elResource(id, label string) string {
	return el(id, label, "tf-resource-name")
}

func elResourceName(id, label string) string {
	return el(id, label, "tf-resource-field-name")
}

func elTfInternalModule(id, label string) string {
	return el(id, label, "tf-int-mod")
}

func elTfExternalModule(id, label string) string {
	return el(id, label, "tf-ext-mod")
}
*/
