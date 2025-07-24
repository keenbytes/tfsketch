package main

import (
	"fmt"
	"html"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func genMermaid(dirs map[string]*Directory, resourceTypeToFind string, outputFile string) {
	mermaidDiagram := &strings.Builder{}

	resourceEdges := &strings.Builder{}

	mermaidDiagram.WriteString(`---
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
`)

	dirKeys := make([]string, len(dirs))
	for dirKey, _ := range dirs {
		dirKeys = append(dirKeys, dirKey)
	}
	sort.Strings(dirKeys)

	for _, dirKey := range dirKeys {
		var dir *Directory
		dir = dirs[dirKey]
		if dir == nil {
			continue
		}

		elementTfPathID := diagramElementID(dir.DisplayPath)
		elementTfPathContents := dir.DisplayPath
		elementTfPath := diagramElementTfPath(elementTfPathID, elementTfPathContents)

		resourceKeys := make([]string, len(dir.Resources))
		for resourceKey, _ := range dir.Resources {
			resourceKeys = append(resourceKeys, resourceKey)
		}
		sort.Strings(resourceKeys)

		for _, resourceKey := range resourceKeys {
			resource := dir.Resources[resourceKey]
			if resource == nil {
				continue
			}

			elementForEachPart := ""
			if resource.ForEach != "" {
				elementForEachPart = "<br>*for_each = " + resource.ForEach + "*"
			}

			elementResourceNameID := elementTfPathID + "___" + diagramElementID(resource.Name)
			elementResourceNameContents := resourceTypeToFind + "." + resource.Name + elementForEachPart
			
			elementResourceName := diagramElementTfResource(elementResourceNameID, elementResourceNameContents, false)

			elementResourceFieldNameID := elementResourceNameID + "___FieldName"
			elementResourceFieldNameContents := resource.FieldName

			elementResourceFieldName := diagramElementTfResourceFieldName(elementResourceFieldNameID, elementResourceFieldNameContents, false)

			_, _ = mermaidDiagram.WriteString(
				fmt.Sprintf(
					"  %s ---> %s --> %s\n",
					elementTfPath,
					elementResourceName,
					elementResourceFieldName,
				),
			)

			resourceEdges.WriteString(elementResourceFieldNameID + "\n")
		}

		writeModulesDiagramCode(mermaidDiagram, dir.Modules, dir.ModulesForEach, elementTfPathID, elementTfPath, resourceTypeToFind, "", "", resourceEdges)
	}

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

func writeModulesDiagramCode(mermaidDiagram *strings.Builder, dirModules map[string]*Directory, dirModulesForEach map[string]string, elementTfPathID string, elementTfPath string, resourceTypeToFind string, parentPath string, parentElementID string, resourceEdges *strings.Builder) {
	for moduleKey, dirModule := range dirModules {
		if dirModule == nil {
			continue
		}

		modKeyValues := strings.Split(moduleKey, ":")
		modResourceName := modKeyValues[0]
		modPath := modKeyValues[1]

		// let's pass the module name as a parent to the next module inside it
		parentPathElement := ""
		if parentPath != "" {
			parentPathElement = parentPath + "<br>-&gt;<br>"
		}
		// new parent path include this element's name
		modPath = strings.TrimRight(modPath, "@")
		modPath = strings.Replace(modPath, "@", `\@`, 1)
		modPath = html.EscapeString(modPath)

		forEachPart := ""
		if dirModulesForEach[moduleKey] != "" {
			forEachPart = "<br><i>for_each = " + html.EscapeString(dirModulesForEach[moduleKey]) + "</i>"
		}

		newParentPathElement := parentPathElement + "<b>module." + modResourceName + "</b><br>" + modPath + forEachPart

		elementModuleContents := newParentPathElement

		elementModuleIDResourceNamePart := "" 
		if parentElementID != "" {
			elementModuleIDResourceNamePart += parentElementID + "___"
		}
		elementModuleIDResourceNamePart += diagramElementID(modResourceName)

		elementModuleID := elementTfPathID + "___mod___" + diagramElementID(dirModule.DisplayPath) + "___" + elementModuleIDResourceNamePart

		elementModulePath := ""
		if parentPath == "" && strings.HasPrefix(modPath, "./modules") {
			elementModulePath = diagramElementTfInternalModule(elementModuleID, elementModuleContents, false)
		} else {
			elementModulePath = diagramElementTfExternalModule(elementModuleID, elementModuleContents, false)
		}

		// do not print a module that has no resources
		if len(dirModule.Resources) > 0 {
			_, _ = mermaidDiagram.WriteString(
				fmt.Sprintf(
					"  %s --> %s\n",
					elementTfPath,
					elementModulePath,
				),
			)
		}

		// looping through module resources
		for _, resource := range dirModule.Resources {
			elementForEachPart := ""
			if resource.ForEach != "" {
				elementForEachPart = "<br><i>for_each = " + html.EscapeString(resource.ForEach) + "</i>"
			}

			elementResourceNameID := elementModuleID + "___" + diagramElementID(resource.Name)
			elementResourceNameContents := resourceTypeToFind + "." + resource.Name + elementForEachPart
			elementResourceName := ""

			elementResourceName = diagramElementTfResource(elementResourceNameID, elementResourceNameContents, false)
			
			elementResourceFieldNameID := elementResourceNameID + "___FieldName"
			elementResourceFieldNameContents := resource.FieldName
			
			elementResourceFieldName := diagramElementTfResourceFieldName(elementResourceFieldNameID, elementResourceFieldNameContents, false)

			_, _ = mermaidDiagram.WriteString(
				fmt.Sprintf(
					"  %s --> %s --> %s\n",
					elementModuleID,
					elementResourceName,
					elementResourceFieldName,
				),
			)

			resourceEdges.WriteString(elementResourceFieldNameID + "\n")
		}

		if len(dirModule.Modules) == 0 {
			continue
		}

		writeModulesDiagramCode(mermaidDiagram, dirModule.Modules, dirModule.ModulesForEach, elementTfPathID, elementTfPath, resourceTypeToFind, newParentPathElement, elementModuleIDResourceNamePart, resourceEdges)
	}
}

func clearString(str string) string {
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}

func diagramElement(elementId, elementContent, classDef string) string {
	return fmt.Sprintf("%s[\"%s\"]:::%s", elementId, elementContent, classDef)
}

func diagramElementTfPath(elementId, elementContent string) string {
	return diagramElement(elementId, elementContent, "tf-path")
}

func diagramElementTfResource(elementId, elementContent string, multiple bool) string {
	return diagramElement(elementId, elementContent, "tf-resource-name" + diagramGetMultipleSuffix(multiple))
}

func diagramElementTfResourceFieldName(elementId, elementContent string, multiple bool) string {
	return diagramElement(elementId, elementContent, "tf-resource-field-name" + diagramGetMultipleSuffix(multiple))
}

func diagramElementTfInternalModule(elementId, elementContent string, multiple bool) string {
	return diagramElement(elementId, elementContent, "tf-int-mod" + diagramGetMultipleSuffix(multiple))
}

func diagramElementTfExternalModule(elementId, elementContent string, multiple bool) string {
	return diagramElement(elementId, elementContent, "tf-ext-mod" + diagramGetMultipleSuffix(multiple))
}

func diagramElementID(text string) string {
	text = strings.ReplaceAll(text, "/", "_")
	text = clearString(text)
	return text
}

func diagramGetMultipleSuffix(multiple bool) string {
	multipleSuffix := ""
	if multiple { 
		multipleSuffix = "@{ shape: procs }"
	}
	return multipleSuffix
}
