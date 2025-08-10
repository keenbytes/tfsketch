// Package chart contains all the code related to generating a diagram.
package chart

import (
	"encoding/json"
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

const (
	elementSeparator = "__"
	partSeparator    = "_"
)

const maxWriteModulesDepth = 5

var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)

const newFilesMode = 0o600

// MermaidFlowChart represents a flowchart.
type MermaidFlowChart struct {
	onlyRoot           bool
	includeFilenames   bool
	minify             bool
	chart              *strings.Builder
	summary            *Summary
	idNum              int
	minifiedElementIDs *map[string]string
}

// NewMermaidFlowChart returns a MermaidFlowChart instance.
func NewMermaidFlowChart(onlyRoot, includeFilenames, minify bool) *MermaidFlowChart {
	minifiedElementIDs := map[string]string{}

	flowchart := &MermaidFlowChart{
		chart:            &strings.Builder{},
		onlyRoot:         onlyRoot,
		includeFilenames: includeFilenames,
		minify:           minify,
		summary:          NewSummary(),
		idNum:            0,
		minifiedElementIDs: &minifiedElementIDs,
	}

	return flowchart
}

// Reset empties the chart.
func (m *MermaidFlowChart) Reset() {
	m.chart.Reset()
	m.summary.Reset()
	m.idNum = 0

	minifiedElementIDs := map[string]string{}
	m.minifiedElementIDs = &minifiedElementIDs
}

// Generate takes a path with Terraform code and generates a chart.
func (m *MermaidFlowChart) Generate(tfPath *tfpath.TfPath, outputFile string) error {
	m.Reset()

	m.chart.WriteString(config)
	m.writePath(tfPath)

	err := os.WriteFile(filepath.Clean(outputFile), []byte(m.chart.String()), newFilesMode)
	if err != nil {
		slog.Error(
			"error writing output file",
			slog.String("path", outputFile),
			slog.String("error", err.Error()),
		)
	}

	summaryFile := outputFile + ".json"

	summaryBytes, err := json.Marshal(m.summary)
	if err != nil {
		slog.Error(
			"error marshaling summary",
			slog.String("path", summaryFile),
			slog.String("error", err.Error()),
		)

		return nil
	}

	err = os.WriteFile(filepath.Clean(summaryFile), summaryBytes, newFilesMode)
	if err != nil {
		slog.Error(
			"error writing summary file",
			slog.String("path", summaryFile),
			slog.String("error", err.Error()),
		)
	}

	return nil
}

func (m *MermaidFlowChart) writePath(tfPath *tfpath.TfPath) {
	elPath, elID := m.pathElement(tfPath)
	_, _ = fmt.Fprintf(m.chart, "  p%s%s\n", partSeparator, elPath)

	// path resources
	m.writePathResources(tfPath, elID, false, false)

	// path modules
	m.writePathModules(tfPath, elID, "", "", false, 1)

	// sub-paths
	if m.onlyRoot {
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

		elChildPath, elChildID := m.pathElement(childTfPath)
		_, _ = fmt.Fprintf(m.chart, "  p%s%s\n", partSeparator, elChildPath)

		// resources
		m.writePathResources(childTfPath, elChildID, false, false)

		// modules
		m.writePathModules(childTfPath, elChildID, "", "", false, 1)
	}
}

func (m *MermaidFlowChart) writePathResources(
	tfPath *tfpath.TfPath,
	elID string,
	isPathModule, forceMultiple bool,
) {
	sortedResources := tfPath.ResourceNamesSorted()
	for _, resourceKey := range sortedResources {
		resource := tfPath.Resources[resourceKey]
		if resource == nil {
			continue
		}

		elResource, elResourceID, _, isMultiple := m.resourceElement(resource, elID)
		if isPathModule {
			_, _ = fmt.Fprintf(
				m.chart,
				"  m%s%s ---> r%s%s\n",
				partSeparator,
				elID,
				partSeparator,
				elResource,
			)
		} else {
			_, _ = fmt.Fprintf(m.chart, "  p%s%s ----> r%s%s\n", partSeparator, elID, partSeparator, elResource)
		}

		elName, elNameID, elNameLabel := m.nameElement(
			resource,
			elResourceID,
			isMultiple || forceMultiple,
		)
		_, _ = fmt.Fprintf(
			m.chart,
			"  r%s%s ---> n%s%s\n",
			partSeparator,
			elResourceID,
			partSeparator,
			elName,
		)

		m.summary.AddEdge(fmt.Sprintf("n%s%s", partSeparator, elNameID))
		m.summary.AddName(elNameLabel)
	}
}

func (m *MermaidFlowChart) writePathModules(
	tfPath *tfpath.TfPath,
	elPathID, elParentModuleID, elParentModuleLabel string,
	forceMultiple bool,
	depth int,
) {
	if depth > maxWriteModulesDepth {
		return
	}

	if tfPath == nil {
		return
	}

	sortedModules := tfPath.ModuleNamesSorted()
	for _, moduleKey := range sortedModules {
		module := tfPath.Modules[moduleKey]
		if module == nil {
			continue
		}

		if !strings.HasPrefix(module.FieldSource, ".") {
			m.summary.AddModule(module.FieldSource + "@" + module.FieldVersion)
		}

		if module.TfPath == nil {
			continue
		}

		elModule, elModuleID, elModuleLabel, isMultiple := m.moduleElement(
			module,
			elPathID,
			elParentModuleID,
			elParentModuleLabel,
		)
		if len(module.TfPath.Resources) > 0 {
			_, _ = fmt.Fprintf(
				m.chart,
				"  p%s%s --> m%s%s\n",
				partSeparator,
				elPathID,
				partSeparator,
				elModule,
			)

			// resources
			m.writePathResources(module.TfPath, elModuleID, true, isMultiple || forceMultiple)
		}

		// modules
		m.writePathModules(
			module.TfPath,
			elPathID,
			elModuleID,
			elModuleLabel,
			isMultiple || forceMultiple,
			depth+1,
		)
	}
}

//nolint:varnamelen
func (m *MermaidFlowChart) pathElement(tfPath *tfpath.TfPath) (string, string) {
	id := m.elementID(tfPath.RelPath)
	label := tfPath.RelPath

	if id == "" {
		id = "root"
	}

	if label == "" {
		label = "."
	}

	return fmt.Sprintf("%s[\"%s\"]:::tf-path", id, label), id
}

//nolint:varnamelen
func (m *MermaidFlowChart) resourceElement(
	resource *tfpath.TfResource,
	elPathID string,
) (string, string, string, bool) {
	id := elPathID + elementSeparator + m.elementID(resource.Name)
	label := fmt.Sprintf("%s.%s", resource.Type, resource.Name)

	isMultiple := false

	if resource.FieldForEach != "" {
		label += "<br>*for_each = " + m.escapeLabel(resource.FieldForEach) + "*"
		isMultiple = true
	}

	if m.includeFilenames {
		label += "<br><i>(" + m.escapeLabel(resource.FilePath) + ")</i>"
	}

	return fmt.Sprintf("%s[\"%s\"]:::tf-resource", id, label), id, label, isMultiple
}

//nolint:varnamelen
func (m *MermaidFlowChart) nameElement(
	resource *tfpath.TfResource,
	elResourceID string,
	isMultiple bool,
) (string, string, string) {
	id := elResourceID + partSeparator + "n"
	label := m.escapeLabel(resource.FieldName)

	if isMultiple {
		return fmt.Sprintf("%s:::tf-name@{ shape: procs, label: \"%s\"}", id, label), id, label
	}

	return fmt.Sprintf("%s[\"%s\"]:::tf-name", id, label), id, label
}

//nolint:varnamelen
func (m *MermaidFlowChart) moduleElement(
	module *tfpath.TfModule,
	elPathID, elParentModuleID, elParentModuleLabel string,
) (string, string, string, bool) {
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
		label += "(at)" + m.escapeLabel(version)
	}

	isMultiple := false

	if module.FieldForEach != "" {
		label += "<br>*for_each = " + m.escapeLabel(module.FieldForEach) + "*"
		isMultiple = true
	}

	if m.includeFilenames {
		label += "<br><i>(" + m.escapeLabel(module.FilePath) + ")</i>"
	}

	return fmt.Sprintf("%s[\"%s\"]:::tf-int-mod", id, label), id, label, isMultiple
}

func (m *MermaidFlowChart) elementID(text string) string {
	text = strings.ReplaceAll(text, "/", "_")
	text = m.removeNonAlphanumericChars(text)

	if !m.minify {
		return text
	}

	minifiedIds := *m.minifiedElementIDs

	minified, exists := minifiedIds[text]
	if exists {
		return minified
	}

	m.idNum++
	minifiedId := fmt.Sprintf("m%d", m.idNum)
	minifiedIds[text] = minifiedId

	return minifiedId
}

func (m *MermaidFlowChart) removeNonAlphanumericChars(str string) string {
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}

func (m *MermaidFlowChart) escapeLabel(label string) string {
	return strings.ReplaceAll(html.EscapeString(label), "&#", "#")
}
