// Package main is the main tool code.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/keenbytes/broccli/v3"
	"github.com/keenbytes/tfsketch/internal/overrides"
	"github.com/keenbytes/tfsketch/pkg/chart"
	"github.com/keenbytes/tfsketch/pkg/tfpath"
)

const (
	exitCodeErrReadingOverridesFromFile = 10
	exitCodeErrTraversingOverrides      = 11
	exitCodeErrParsingContainerPaths    = 21
	exitCodeErrLinkingContainerPaths    = 22
	exitCodeErrParsingTerraformPath     = 31
	exitCodeErrLinkingTerraformPath     = 32
	exitCodeErrGeneratingChart          = 41
)

func main() {
	cli := broccli.NewBroccli(
		"tfsketch",
		"Generate diagram from Terraform files",
		"Mikolaj Gasior <m@gasior.dev>",
	)

	cmd := cli.Command("gen", "Generate diagram", genHandler)
	cmd.Arg(
		"path",
		"DIR",
		"Path to directory with terraform code",
		broccli.TypePathFile,
		broccli.IsDirectory|broccli.IsExistent|broccli.IsRequired,
	)
	cmd.Arg("output", "FILE", "Path to an output file", broccli.TypePathFile, broccli.IsRequired)
	cmd.Flag(
		"type-regexp",
		"t",
		"REGEXP",
		"Regular expression to filter type of the resource",
		broccli.TypeString,
		0,
	)
	cmd.Flag(
		"overrides",
		"o",
		"FILE",
		"YAML file mapping external modules to local paths",
		broccli.TypePathFile,
		broccli.IsRegularFile,
	)
	cmd.Flag("debug", "d", "", "Enable debug mode", broccli.TypeBool, 0)
	cmd.Flag("only-root", "d1", "", "Draw only root directory", broccli.TypeBool, 0)
	cmd.Flag(
		"include-filenames",
		"d2",
		"",
		"Display source filenames on the diagram",
		broccli.TypeBool,
		0,
	)
	cmd.Flag(
		"minify",
		"d3",
		"",
		"Minify element names in the chart to save space",
		broccli.TypeBool,
		0,
	)

	os.Exit(cli.Run(context.Background()))
}

//nolint:funlen
func genHandler(_ context.Context, cli *broccli.Broccli) int {
	slog.Info("🚀 tfsketch starting...")

	setLogger(cli.Flag("debug"))
	terraformPath, resourceType, outputFile, overridesPath, onlyRoot, includeFilenames, minify := getGenArgsAndFlags(
		cli,
	)

	container := tfpath.NewContainer()

	traverser := tfpath.NewTraverser(container, resourceType)

	var err error

	// overrides
	if overridesPath != "" {
		overrides := &overrides.Overrides{}

		err = overrides.ReadFromFile(overridesPath)
		if err != nil {
			slog.Error("❌ Error reading overrides from file: " + err.Error())

			return exitCodeErrReadingOverridesFromFile
		}

		externalModulesNum := len(overrides.ExternalModules)
		slog.Info(
			fmt.Sprintf("🔸 External modules number in overrides file: %d", externalModulesNum),
		)

		for _, externalModule := range overrides.ExternalModules {
			tfPath := tfpath.NewTfPath(externalModule.Local, externalModule.Remote)
			container.AddPath(tfPath.TraverseName, tfPath)

			isSubModule := isExternalModuleASubModule(externalModule.Remote)

			err := traverser.WalkPath(tfPath, !isSubModule)
			if err != nil {
				slog.Error(
					fmt.Sprintf(
						"❌ Error walking dirs in overrides local path 📁%s: %s",
						externalModule.Local,
						err.Error(),
					),
				)

				return exitCodeErrTraversingOverrides
			}
		}
	}

	// as of now, use paths in container
	for pathName, tfPath := range container.Paths {
		err := traverser.ParsePath(tfPath)
		if err != nil {
			slog.Error(
				fmt.Sprintf(
					"❌ Error parsing container terraform path 📁%s (%s) : %s",
					tfPath.Path,
					pathName,
					err.Error(),
				),
			)

			return exitCodeErrParsingContainerPaths
		}
	}

	for pathName, tfPath := range container.Paths {
		err = traverser.LinkPath(tfPath)
		if err != nil {
			slog.Error(
				fmt.Sprintf(
					"❌ Error linking local modules in terraform path 📁%s (%s) : %s",
					tfPath.Path,
					pathName,
					err.Error(),
				),
			)

			return exitCodeErrLinkingContainerPaths
		}
	}

	// path
	rootTfPathName := "."
	rootTfPath := tfpath.NewTfPath(terraformPath, rootTfPathName)
	container.AddPath(rootTfPathName, rootTfPath)

	err = traverser.WalkPath(rootTfPath, false)
	if err != nil {
		slog.Error(
			fmt.Sprintf(
				"❌ Error walking dirs in terraform path 📁%s: %s",
				rootTfPath.Path,
				err.Error(),
			),
		)

		return exitCodeErrTraversingOverrides
	}

	err = traverser.ParsePath(rootTfPath)
	if err != nil {
		slog.Error(
			fmt.Sprintf(
				"❌ Error parsing terraform path 📁%s (%s) : %s",
				rootTfPath.Path,
				rootTfPathName,
				err.Error(),
			),
		)

		return exitCodeErrParsingTerraformPath
	}

	err = traverser.LinkPath(rootTfPath)
	if err != nil {
		slog.Error(
			fmt.Sprintf(
				"❌ Error linking local modules in terraform path 📁%s (%s) : %s",
				rootTfPath.Path,
				rootTfPathName,
				err.Error(),
			),
		)

		return exitCodeErrLinkingTerraformPath
	}

	flowchart := chart.NewMermaidFlowChart(onlyRoot, includeFilenames, minify)

	err = flowchart.Generate(rootTfPath, outputFile)
	if err != nil {
		slog.Error(
			fmt.Sprintf(
				"❌ Error generating chart from terraform path 📁%s (%s) : %s",
				rootTfPath.Path,
				rootTfPathName,
				err.Error(),
			),
		)

		return exitCodeErrGeneratingChart
	}

	return 0
}

//nolint:goconst
func getGenArgsAndFlags(cli *broccli.Broccli) (string, string, string, string, bool, bool, bool) {
	terraformPath := cli.Arg("path")
	outputFile := cli.Arg("output")
	resourceType := cli.Flag("type-regexp")
	overrides := cli.Flag("overrides")
	onlyRoot := cli.Flag("only-root")
	includeFilenames := cli.Flag("include-filenames")
	minify := cli.Flag("minify")

	if resourceType == "" {
		resourceType = "^.*$"
	}

	slog.Info("✨ Terraform path to scan:          " + terraformPath)
	slog.Info("✨ Resource type regexp:            " + resourceType)
	slog.Info("✨ Output diagram destination:      " + outputFile)
	slog.Info("✨ External modules overrides file: " + overrides)
	slog.Info("✨ Draw only root path:             " + onlyRoot)
	slog.Info("✨ Include source filename:         " + includeFilenames)
	slog.Info("✨ Minify element names:            " + minify)

	return terraformPath, resourceType, outputFile, overrides, onlyRoot == "true", includeFilenames == "true", minify == "true"
}

func setLogger(debug string) {
	logLevel := slog.LevelInfo
	if debug == "true" {
		logLevel = slog.LevelDebug
	}

	slog.SetLogLoggerLevel(logLevel)
}

func isExternalModuleASubModule(module string) bool {
	return strings.Contains(module, "//modules/")
}
