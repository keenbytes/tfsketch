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
    nodeSpacing: 10
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
			
			multiple := false
			if resource.ForEach != "" {
				multiple = true
			}

			elementResourceName := diagramElementTfResource(elementResourceNameID, elementResourceNameContents, multiple)

			elementResourceFieldNameID := elementResourceNameID + "___FieldName"
			elementResourceFieldNameContents := resource.FieldName

			elementResourceFieldName := diagramElementTfResourceFieldName(elementResourceFieldNameID, elementResourceFieldNameContents, multiple)

			_, _ = mermaidDiagram.WriteString(
				fmt.Sprintf(
					"  %s ---> %s --> %s\n",
					elementTfPath,
					elementResourceName,
					elementResourceFieldName,
				),
			)
		}

		writeModulesDiagramCode(mermaidDiagram, dir.Modules, dir.ModulesForEach, elementTfPathID, elementTfPath, resourceTypeToFind, "", "", false)
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

func writeModulesDiagramCode(mermaidDiagram *strings.Builder, dirModules map[string]*Directory, dirModulesForEach map[string]string, elementTfPathID string, elementTfPath string, resourceTypeToFind string, parentPath string, parentElementID string, multiple bool) {
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

		if parentPath == "" && dirModulesForEach[moduleKey] != "" {
			multiple = true
		}

		elementModulePath := ""
		if parentPath == "" && strings.HasPrefix(modPath, "./modules") {
			elementModulePath = diagramElementTfInternalModule(elementModuleID, elementModuleContents, multiple)
		} else {
			elementModulePath = diagramElementTfExternalModule(elementModuleID, elementModuleContents, multiple)
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
			
			multipleResource := false
			if multiple {
				multipleResource = true
			}

			if !multipleResource && resource.ForEach != "" {
				multipleResource = true
			}

			elementResourceName = diagramElementTfResource(elementResourceNameID, elementResourceNameContents, multipleResource)
			
			elementResourceFieldNameID := elementResourceNameID + "___FieldName"
			elementResourceFieldNameContents := resource.FieldName
			
			elementResourceFieldName := diagramElementTfResourceFieldName(elementResourceFieldNameID, elementResourceFieldNameContents, multipleResource)

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

		writeModulesDiagramCode(mermaidDiagram, dirModule.Modules, dirModule.ModulesForEach, elementTfPathID, elementTfPath, resourceTypeToFind, newParentPathElement, elementModuleIDResourceNamePart, multiple)
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
