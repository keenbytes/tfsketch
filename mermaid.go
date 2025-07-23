package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func genMermaid(dirs map[string]*Directory, resourceTypeToFind string) {
	mermaidDiagram := &strings.Builder{}

	mermaidDiagram.WriteString(`---
config:
  theme: redux
---
flowchart LR
  classDef tf-path fill:#c87de8
  classDef tf-resource-name stroke:#e7b6fc,color:#c87de8
	classDef tf-resource-name-int-mod fill:#e7b6fc
	classDef tf-resource-name-ext-mod fill:#7da8e8
  classDef tf-resource-field-name fill:#eb91c7
`)

	for _, dir := range dirs {
		elementTfPathID := diagramElementID(dir.DisplayPath)
		elementTfPathContents := "Path: " + dir.DisplayPath
		elementTfPath := diagramElementTfPath(elementTfPathID, elementTfPathContents)

		for _, resource := range dir.Resources {
			elementResourceNameID := elementTfPathID + "___" + diagramElementID(resource.Name)
			elementResourceNameContents := "Resource: " + resourceTypeToFind + "." + resource.Name
			elementResourceName := diagramElementTfResource(elementResourceNameID, elementResourceNameContents)

			elementResourceFieldNameID := elementResourceNameID + "___FieldName"
			elementResourceFieldNameContents := resource.FieldName
			elementResourceFieldName := diagramElementTfResourceFieldName(elementResourceFieldNameID, elementResourceFieldNameContents)

			_, _ = mermaidDiagram.WriteString(
				fmt.Sprintf(
					"  %s --> %s --> %s\n",
					elementTfPath,
					elementResourceName,
					elementResourceFieldName,
				),
			)
		}

		writeModulesDiagramCode(mermaidDiagram, dir.Modules, elementTfPathID, elementTfPath, resourceTypeToFind)
	}

	fmt.Fprint(os.Stdout, mermaidDiagram.String())
}

func writeModulesDiagramCode(mermaidDiagram *strings.Builder, dirModules map[string]*Directory, elementTfPathID string, elementTfPath string, resourceTypeToFind string) {
	for moduleKey, dirModule := range dirModules {
		if dirModule == nil {
			continue
		}

		modKeyValues := strings.Split(moduleKey, ":")
		modResourceName := modKeyValues[0]
		modPath := modKeyValues[1]

		elementModuleID := elementTfPathID + "___mod___" + diagramElementID(dirModule.DisplayPath) + "___" + diagramElementID(modResourceName)
		elementModuleContents := "Resource: module." + modResourceName + " > " + resourceTypeToFind + "."

		// looping through module resources
		for _, resource := range dirModule.Resources {
			elementResourceNameID := elementModuleID + "___" + diagramElementID(resource.Name)
			elementResourceNameContents := elementModuleContents + resource.Name
			elementResourceName := ""
			
			if strings.HasPrefix(modPath, "./modules") {
				elementResourceName = diagramElementTfResourceFromInternalModule(elementResourceNameID, elementResourceNameContents)
			} else {
				elementResourceName = diagramElementTfResourceFromExternalModule(elementResourceNameID, elementResourceNameContents)
			}

			elementResourceFieldNameID := elementResourceNameID + "___FieldName"
			elementResourceFieldNameContents := resource.FieldName
			elementResourceFieldName := diagramElementTfResourceFieldName(elementResourceFieldNameID, elementResourceFieldNameContents)

			_, _ = mermaidDiagram.WriteString(
				fmt.Sprintf(
					"  %s --> %s --> %s\n",
					elementTfPath,
					elementResourceName,
					elementResourceFieldName,
				),
			)
		}

		if len(dirModule.Modules) == 0 {
			continue
		}

		writeModulesDiagramCode(mermaidDiagram, dirModule.Modules, elementTfPathID, elementTfPath, resourceTypeToFind)
	}
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)

func clearString(str string) string {
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}

func diagramElement(elementId, elementContent, classDef string) string {
	return fmt.Sprintf("%s[\"%s\"]:::%s", elementId, elementContent, classDef)
}

func diagramElementTfPath(elementId, elementContent string) string {
	return diagramElement(elementId, elementContent, "tf-path")
}

func diagramElementTfResource(elementId, elementContent string) string {
	return diagramElement(elementId, elementContent, "tf-resource-name")
}

func diagramElementTfResourceFieldName(elementId, elementContent string) string {
	return diagramElement(elementId, elementContent, "tf-resource-field-name")
}

func diagramElementTfResourceFromInternalModule(elementId, elementContent string) string {
	return diagramElement(elementId, elementContent, "tf-resource-name-int-mod")
}

func diagramElementTfResourceFromExternalModule(elementId, elementContent string) string {
	return diagramElement(elementId, elementContent, "tf-resource-name-ext-mod")
}

func diagramElementID(text string) string {
	text = strings.ReplaceAll(text, "/", "_")
	text = clearString(text)
	return text
}
