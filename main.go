// Package main contains CLI commands definition for tfsketch.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"tfsketch/internal/chart"
	"tfsketch/internal/overrides"
	"tfsketch/internal/tfpath"
)

const (
	exitCodeErrReadingOverridesFromFile = 10
	exitCodeErrTraversingOverrides      = 11
	exitCodeErrParsingContainerPaths    = 21
	exitCodeErrLinkingContainerPaths    = 22
	exitCodeErrGeneratingChart          = 41
)

//nolint:funlen
func main() {
	rootCmd := &cobra.Command{
		Use:   "tfsketch",
		Short: "Generate diagram from Terraform files",
		Long:  "tfsketch generates diagrams from Terraform files",
	}

	var terraformPath string
	var outputFile string
	var pathIncludeRegexp, pathExcludeRegexp, typeRegexp, nameRegexp, displayAttributes string
	var overridesPath, cachePath string
	var debug, onlyRoot, includeFilenames, minify, module bool

	genCmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate diagram",
		Long:  "Generate diagrams based on Terraform files",
		Run: func(cmd *cobra.Command, args []string) {
			os.Exit(genHandler(cmd.Context(), debug, terraformPath, pathIncludeRegexp, pathExcludeRegexp, typeRegexp, nameRegexp, displayAttributes, outputFile, overridesPath, cachePath, onlyRoot, includeFilenames, minify, module))
		},
	}

	genCmd.PersistentFlags().StringVarP(&terraformPath, "path", "", "", "Path to directory with terraform code (required)")
	genCmd.MarkPersistentFlagRequired("path")
	genCmd.MarkPersistentFlagDirname("path")

	genCmd.PersistentFlags().StringVarP(&outputFile, "output", "", "", "Path to an output file (required)")
	genCmd.MarkPersistentFlagRequired("output")
	genCmd.MarkPersistentFlagFilename("output")

	genCmd.Flags().StringVarP(&pathIncludeRegexp, "path-include-regexp", "i", "^.*$", "Regular expression to include paths")
	genCmd.Flags().StringVarP(&pathExcludeRegexp, "path-exclude-regexp", "e", "^SillyName$", "Regular expression to exclude paths")
	genCmd.Flags().StringVarP(&typeRegexp, "type-regexp", "t", "^.*$", "Regular expression to filter type of the resource")
	genCmd.Flags().StringVarP(&nameRegexp, "name-regexp", "n", "^.*$", "Regular expression to filter name of the resource")

	genCmd.Flags().StringVarP(
		&displayAttributes, "display-attributes", "a", "",
		"Comma-separated resource attributes; the first found is used as the chart’s display name",
	)
	genCmd.Flags().StringVarP(&overridesPath, "overrides", "o", "", "YAML file mapping external modules to local paths")
	genCmd.Flags().StringVarP(&cachePath, "cache", "c", "", "Path to directory where modules will be downloaded and cached")

	genCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug mode")
	genCmd.Flags().BoolVarP(&onlyRoot, "only-root", "r", false, "Draw only root directory")
	genCmd.Flags().BoolVarP(&includeFilenames, "include-filenames", "f", false, "Display source filenames on the diagram")
	genCmd.Flags().BoolVarP(&minify, "minify", "s", false, "Minify element names in the chart to save space")
	genCmd.Flags().BoolVarP(&module, "module", "m", false, "Treat path as module and draw 'modules' sub-directory")
	rootCmd.AddCommand(genCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Command execution error:", err)
		os.Exit(1)
	}
}

//nolint:funlen
func genHandler(_ context.Context, debug bool, terraformPath, pathIncludeRegexp, pathExcludeRegexp, typeRegexp, nameRegexp,
	displayAttributes, outputFile, overridesPath, cachePath string, onlyRoot, includeFilenames,
	minify, module bool) int {
	slog.Info("🚀 tfsketch starting...")

	if typeRegexp == "" {
		typeRegexp = "^.*$"
	}

	if nameRegexp == "" {
		nameRegexp = "^.*$"
	}

	if pathIncludeRegexp == "" {
		pathIncludeRegexp = "^.*$"
	}

	if pathExcludeRegexp == "" {
		pathExcludeRegexp = "^SillyName$"
	}

	slog.Info("✨ Terraform path to scan:          " + terraformPath)
	slog.Info("✨ Include path regexp:             " + pathIncludeRegexp)
	slog.Info("✨ Exclude path regexp:             " + pathExcludeRegexp)
	slog.Info("✨ Resource type regexp:            " + typeRegexp)
	slog.Info("✨ Resource name regexp:            " + nameRegexp)
	slog.Info("✨ Display attributes:              " + displayAttributes)
	slog.Info("✨ Output diagram destination:      " + outputFile)
	slog.Info("✨ External modules overrides file: " + overridesPath)
	slog.Info("✨ Draw only root path:             " + fmt.Sprintf("%v", onlyRoot))
	slog.Info("✨ Include source filename:         " + fmt.Sprintf("%v", includeFilenames))
	slog.Info("✨ Minify element names:            " + fmt.Sprintf("%v", minify))
	slog.Info("✨ Draw 'modules' sub-directory:    " + fmt.Sprintf("%v", module))
	slog.Info("✨ Cache path:                      " + cachePath)

	setLogger(debug)

	var cache *tfpath.Cache
	if cachePath != "" {
		cache = tfpath.NewCache(cachePath)
	}

	container := tfpath.NewContainer()

	traverser := tfpath.NewTraverser(
		container,
		pathIncludeRegexp,
		pathExcludeRegexp,
		typeRegexp,
		nameRegexp,
		displayAttributes,
		cache,
	)

	var err error

	// overrides
	if overridesPath != "" {
		overrides := &overrides.Overrides{}

		err := overrides.ReadFromFile(overridesPath)
		if err != nil {
			slog.Error("❌ Error reading overrides from file: " + err.Error())

			return exitCodeErrReadingOverridesFromFile
		}

		err = container.WalkOverrides(overrides, traverser, cache)
		if err != nil {
			return exitCodeErrTraversingOverrides
		}

		externalModulesNum := len(overrides.ExternalModules)
		slog.Info(
			fmt.Sprintf("🔸 External modules number in overrides file: %d", externalModulesNum),
		)
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

	// as of now, use paths in container
	err = container.ParsePaths(traverser, cache, 1)
	if err != nil {
		return exitCodeErrParsingContainerPaths
	}

	err = container.LinkPaths(traverser)
	if err != nil {
		return exitCodeErrLinkingContainerPaths
	}

	flowchart := chart.NewMermaidFlowChart(onlyRoot, includeFilenames, minify, module)

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

func setLogger(debug bool) {
	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}

	slog.SetLogLoggerLevel(logLevel)
}
