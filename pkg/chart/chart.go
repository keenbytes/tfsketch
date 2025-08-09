package chart

import (
	"fmt"
	"html"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/keenbytes/tfsketch/pkg/tfpath"
)

const config = `---
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
  classDef tf-resource stroke:#e7b6fc,color:#c87de8,text-align:left
  classDef tf-int-mod fill:#e7b6fc,text-align:left
  classDef tf-ext-mod fill:#7da8e8,text-align:left
  classDef tf-name fill:#eb91c7
`

const elementSeparator = "__"
const partSeparator = "_"

const maxWriteModulesDepth = 5

var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)

type MermaidFlowChart struct{
  OnlyRoot bool
  IncludeFilenames bool
}

func (m MermaidFlowChart) Generate(tfPath *tfpath.TfPath, resourceType string, outputFile string) error {
  // output chart and list of resource edges
  chart := &strings.Builder{}
  edges := &strings.Builder{}

  chart.WriteString(config)
  m.writePath(tfPath, chart, edges)

	err := os.WriteFile(filepath.Clean(outputFile), []byte(chart.String()), 0600)
	if err != nil {
		slog.Error(
			"error writing output file",
			slog.String("path", outputFile),
			slog.String("error", err.Error()),
		)
	}

	edgesFile := outputFile + ".edges.txt"
	err = os.WriteFile(filepath.Clean(edgesFile), []byte(edges.String()), 0600)
	if err != nil {
		slog.Error(
			"error writing file with edges",
			slog.String("path", edgesFile),
			slog.String("error", err.Error()),
		)
	}

  return nil
}

func (m MermaidFlowChart) writePath(tfPath *tfpath.TfPath, chart, edges *strings.Builder) {
  elPath, elID, _ := m.pathElement(tfPath)
  _, _ = chart.WriteString(fmt.Sprintf("  p%s%s\n", partSeparator, elPath))

  // path resources
  m.writePathResources(tfPath, elID, false, false, chart, edges)

  // path modules
  m.writePathModules(tfPath, elID, "", "", false, chart, edges, 1)

  // sub-paths
  if m.OnlyRoot {
    return
  }

  sortedPaths := tfPath.ChildrenNamesSorted()
  for _, childKey := range sortedPaths {
    childTfPath := tfPath.Children[childKey]
    if childTfPath == nil {
      continue
    }

    // not module
    _, isModule := tfPath.IsChildModule[childKey]
    if isModule {
      continue
    }

    // only first-depth sub-directories
    if strings.Contains(childTfPath.RelPath, "/") {
      continue
    }

    elChildPath, elChildID, _ := m.pathElement(childTfPath)
    _, _ = chart.WriteString(fmt.Sprintf("  p%s%s\n", partSeparator, elChildPath))

    // resources
    m.writePathResources(childTfPath, elChildID, false, false, chart, edges)

    // modules
    m.writePathModules(childTfPath, elChildID, "", "", false, chart, edges, 1)
  }

}

func (m MermaidFlowChart) writePathResources(tfPath *tfpath.TfPath, elID string, isPathModule, forceMultiple bool, chart, edges *strings.Builder) {
  sortedResources := tfPath.ResourceNamesSorted()
  for _, resourceKey := range sortedResources {
    resource := tfPath.Resources[resourceKey]
    if resource == nil {
      continue
    }

    elResource, elResourceID, _, isMultiple := m.resourceElement(resource, elID)
    if isPathModule {
      _, _ = chart.WriteString(fmt.Sprintf("  m%s%s ---> r%s%s\n", partSeparator, elID, partSeparator, elResource))
    } else {
      _, _ = chart.WriteString(fmt.Sprintf("  p%s%s ----> r%s%s\n", partSeparator, elID, partSeparator, elResource))
    }

    elName, elNameID, _ := m.nameElement(resource, elResourceID, isMultiple || forceMultiple)
    _, _ = chart.WriteString(fmt.Sprintf("  r%s%s ---> n%s%s\n", partSeparator, elResourceID, partSeparator, elName))

    _, _ = edges.WriteString(fmt.Sprintf("n%s%s\n", partSeparator, elNameID))
  }
}

func (m MermaidFlowChart) writePathModules(tfPath *tfpath.TfPath, elPathID, elParentModuleID, elParentModuleLabel string, forceMultiple bool, chart, edges *strings.Builder, depth int) {
  if depth > maxWriteModulesDepth {
    return
  }

  sortedModules := tfPath.ModuleNamesSorted()
  for _, moduleKey := range sortedModules {
    module := tfPath.Modules[moduleKey]
    if module == nil {
      continue
    }

    elModule, elModuleID, elModuleLabel, isMultiple := m.moduleElement(module, elPathID, elParentModuleID, elParentModuleLabel)
    if len(module.TfPath.Resources) > 0 {
      _, _ = chart.WriteString(fmt.Sprintf("  p%s%s --> m%s%s\n", partSeparator, elPathID, partSeparator, elModule))

      // resources
      m.writePathResources(module.TfPath, elModuleID, true, isMultiple || forceMultiple, chart, edges)
    }

    // modules
    m.writePathModules(module.TfPath, elPathID, elModuleID, elModuleLabel, isMultiple || forceMultiple, chart, edges, depth+1)
  }
}

func (m MermaidFlowChart) pathElement(tfPath *tfpath.TfPath) (string, string, string) {
  id := m.elementID(tfPath.RelPath)
  label := tfPath.RelPath

  if id == "" {
    id = "root"
  }

  if label == "" {
    label = "."
  }

  return fmt.Sprintf("%s[\"%s\"]:::tf-path", id, label), id, label
}

func (m MermaidFlowChart) resourceElement(resource *tfpath.TfResource, elPathID string) (string, string, string, bool) {
  id := elPathID + elementSeparator + m.elementID(resource.Name)
  label := fmt.Sprintf("%s.%s", resource.Type, resource.Name)

  isMultiple := false
  if resource.FieldForEach != "" {
    label += "<br>*for_each = " + m.escapeLabel(resource.FieldForEach) + "*"
    isMultiple = true
  }

  if m.IncludeFilenames {
    label += "<br><i>(" + m.escapeLabel(resource.FilePath) + ")</i>"
  }

  return fmt.Sprintf("%s[\"%s\"]:::tf-resource", id, label), id, label, isMultiple
}

func (m MermaidFlowChart) nameElement(resource *tfpath.TfResource, elResourceID string, isMultiple bool) (string, string, string) {
  id := elResourceID + partSeparator + "n"
  label := m.escapeLabel(resource.FieldName)

  if isMultiple {
    return fmt.Sprintf("%s:::tf-name@{ shape: procs, label: \"%s\"}", id, label), id, label
  } else {
    return fmt.Sprintf("%s[\"%s\"]:::tf-name", id, label), id, label
  }
}

func (m MermaidFlowChart) moduleElement(module *tfpath.TfModule, elPathID, elParentModuleID, elParentModuleLabel string) (string, string, string, bool) {
  id := elPathID + elementSeparator
  if elParentModuleID != "" {
    id += elParentModuleID + elementSeparator
  }
  id += m.elementID(module.Name)

  source := module.FieldSource
  version := module.FieldVersion
  
  var label string
  if elParentModuleLabel != "" {
    label += elParentModuleLabel + "<br><b>/</b><br>"
  }
  label += fmt.Sprintf("module.%s<br>%s", module.Name, m.escapeLabel(source))
  if !strings.HasPrefix(source, ".") {
    label += fmt.Sprintf("(at)%s", m.escapeLabel(version))
  }

  isMultiple := false
  if module.FieldForEach != "" {
    label += "<br>*for_each = " + m.escapeLabel(module.FieldForEach) + "*"
    isMultiple = true
  }


  if m.IncludeFilenames {
    label += "<br><i>(" + m.escapeLabel(module.FilePath) + ")</i>"
  }

  return fmt.Sprintf("%s[\"%s\"]:::tf-int-mod", id, label), id, label, isMultiple
}

func (m MermaidFlowChart) elementID(text string) string {
	text = strings.ReplaceAll(text, "/", "_")
	text = m.removeNonAlphanumericChars(text)
	return text
}

func (m MermaidFlowChart) removeNonAlphanumericChars(str string) string {
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}

func (m MermaidFlowChart) escapeLabel(label string) string {
  return strings.ReplaceAll(html.EscapeString(label), "&#", "#")
}
