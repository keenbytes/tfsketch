package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func genMermaid(dirs map[string]*Directory, resourceTypeToFind string, outputFile string) {
	mermaidDiagram := &strings.Builder{}

	mermaidDiagram.WriteString(`---
config:
  theme: redux
  flowchart:
    diagramPadding: 5
    padding: 5
    nodeSpacing: 5
    wrappingWidth: 700
---
flowchart LR
  classDef tf-path fill:#c87de8
  classDef tf-resource-name stroke:#e7b6fc,color:#c87de8
  classDef tf-int-mod fill:#e7b6fc
  classDef tf-ext-mod fill:#7da8e8
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
		elementTfPathContents := "Path: " + dir.DisplayPath
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

			elementResourceNameID := elementTfPathID + "___" + diagramElementID(resource.Name)
			elementResourceNameContents := "Resource: " + resourceTypeToFind + "." + resource.Name
			elementResourceName := diagramElementTfResource(elementResourceNameID, elementResourceNameContents)

			elementResourceFieldNameID := elementResourceNameID + "___FieldName"
			elementResourceFieldNameContents := resource.FieldName
			elementResourceFieldName := diagramElementTfResourceFieldName(elementResourceFieldNameID, elementResourceFieldNameContents)

			_, _ = mermaidDiagram.WriteString(
				fmt.Sprintf(
					"  %s ---> %s --> %s\n",
					elementTfPath,
					elementResourceName,
					elementResourceFieldName,
				),
			)
		}

		writeModulesDiagramCode(mermaidDiagram, dir.Modules, elementTfPathID, elementTfPath, resourceTypeToFind, "", "")
	}

	err := os.WriteFile(filepath.Clean(outputFile), []byte(mermaidDiagram.String()), 0600)
	if err != nil {
		slog.Error(
			"error writing output file",
			slog.String("path", outputFile),
			slog.String("error", err.Error()),
		)
	}
}

func writeModulesDiagramCode(mermaidDiagram *strings.Builder, dirModules map[string]*Directory, elementTfPathID string, elementTfPath string, resourceTypeToFind string, parentPath string, parentElementID string) {
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
			parentPathElement = parentPath + " > "
		}
		// new parent path include this element's name
		newParentPathElement := parentPathElement + "module." + modResourceName

		elementModuleContents := "Module: " + newParentPathElement

		elementModuleIDResourceNamePart := "" 
		if parentElementID != "" {
			elementModuleIDResourceNamePart += parentElementID + "___"
		}
		elementModuleIDResourceNamePart += diagramElementID(modResourceName)

		elementModuleID := elementTfPathID + "___mod___" + diagramElementID(dirModule.DisplayPath) + "___" + elementModuleIDResourceNamePart

		elementModulePath := ""
		if parentPath == "" && strings.HasPrefix(modPath, "./modules") {
			elementModulePath = diagramElementTfInternalModule(elementModuleID, elementModuleContents)
		} else {
			elementModulePath = diagramElementTfExternalModule(elementModuleID, elementModuleContents)
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
			elementResourceNameID := elementModuleID + "___" + diagramElementID(resource.Name)
			elementResourceNameContents := "Resource: " + resource.Name
			elementResourceName := ""
			
			// only first level modules can be displayed as internal
			if parentPath == "" && strings.HasPrefix(modPath, "./modules") {
				elementResourceName = diagramElementTfResource(elementResourceNameID, elementResourceNameContents)
			} else {
				elementResourceName = diagramElementTfResource(elementResourceNameID, elementResourceNameContents)
			}

			elementResourceFieldNameID := elementResourceNameID + "___FieldName"
			elementResourceFieldNameContents := resource.FieldName
			elementResourceFieldName := diagramElementTfResourceFieldName(elementResourceFieldNameID, elementResourceFieldNameContents)

			_, _ = mermaidDiagram.WriteString(
				fmt.Sprintf(
					"  %s --> %s --> %s\n",
					elementModulePath,
					elementResourceName,
					elementResourceFieldName,
				),
			)
		}

		if len(dirModule.Modules) == 0 {
			continue
		}

		writeModulesDiagramCode(mermaidDiagram, dirModule.Modules, elementTfPathID, elementTfPath, resourceTypeToFind, newParentPathElement, elementModuleIDResourceNamePart)
	}
}

func clearString(str string) string {
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}

func diagramElement(elementId, elementContent, classDef string) string {
	return fmt.Sprintf("%s[\"%s\"]:::%s", elementId, elementContent, classDef)
}

func diagramElementParallelogram(elementId, elementContent, classDef string) string {
	return fmt.Sprintf("%s[/\"%s\"/]:::%s", elementId, elementContent, classDef)
}

func diagramElementAsymmetric(elementId, elementContent, classDef string) string {
	return fmt.Sprintf("%s>\"%s\"]:::%s", elementId, elementContent, classDef)
}

func diagramElementTfPath(elementId, elementContent string) string {
	return diagramElementParallelogram(elementId, elementContent, "tf-path")
}

func diagramElementTfResource(elementId, elementContent string) string {
	return diagramElement(elementId, elementContent, "tf-resource-name")
}

func diagramElementTfResourceFieldName(elementId, elementContent string) string {
	return diagramElement(elementId, elementContent, "tf-resource-field-name")
}

func diagramElementTfInternalModule(elementId, elementContent string) string {
	return diagramElementParallelogram(elementId, elementContent, "tf-int-mod")
}

func diagramElementTfExternalModule(elementId, elementContent string) string {
	return diagramElementAsymmetric(elementId, elementContent, "tf-ext-mod")
}

func diagramElementID(text string) string {
	text = strings.ReplaceAll(text, "/", "_")
	text = clearString(text)
	return text
}
