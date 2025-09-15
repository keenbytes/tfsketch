// Package main contains CLI commands definition for tfsketch.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

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
	exitCodeErrGeneratingChart          = 41
)

//nolint:funlen
func main() {
	cli := broccli.NewBroccli(
		"tfsketch",
		"Generate diagram from Terraform files",
		"Mikołaj Gąsior <m@gasior.dev>",
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
		"path-include-regexp",
		"i",
		"REGEXP",
		"Regular expression to include paths",
		broccli.TypeString,
		0,
	)
	cmd.Flag(
		"path-exclude-regexp",
		"e",
		"REGEXP",
		"Regular expression to exclude paths",
		broccli.TypeString,
		0,
	)
	cmd.Flag(
		"type-regexp",
		"t",
		"REGEXP",
		"Regular expression to filter type of the resource",
		broccli.TypeString,
		0,
	)
	cmd.Flag(
		"name-regexp",
		"n",
		"REGEXP",
		"Regular expression to filter name of the resource",
		broccli.TypeString,
		0,
	)
	cmd.Flag(
		"display-attributes",
		"a",
		"ATTR1,ATTR2,...",
		"Comma-separated resource attributes; the first found is used as the chart’s display name",
		broccli.TypeAlphanumeric,
		broccli.AllowHyphen|broccli.AllowUnderscore|broccli.AllowMultipleValues,
	)
	cmd.Flag(
		"overrides",
		"o",
		"FILE",
		"YAML file mapping external modules to local paths",
		broccli.TypePathFile,
		broccli.IsRegularFile,
	)
	cmd.Flag(
		"cache",
		"c",
		"DIR",
		"Path to directory where modules will be downloaded and cached",
		broccli.TypePathFile,
		broccli.IsDirectory|broccli.IsExistent,
	)
	cmd.Flag("debug", "d", "", "Enable debug mode", broccli.TypeBool, 0)
	cmd.Flag("only-root", "r", "", "Draw only root directory", broccli.TypeBool, 0)
	cmd.Flag(
		"include-filenames",
		"f",
		"",
		"Display source filenames on the diagram",
		broccli.TypeBool,
		0,
	)
	cmd.Flag(
		"minify",
		"s",
		"",
		"Minify element names in the chart to save space",
		broccli.TypeBool,
		0,
	)
	cmd.Flag(
		"module",
		"m",
		"",
		"Treat path as module and draw 'modules' sub-directory",
		broccli.TypeBool,
		0,
	)

	os.Exit(cli.Run(context.Background()))
}

//nolint:funlen
func genHandler(_ context.Context, cli *broccli.Broccli) int {
	slog.Info("🚀 tfsketch starting...")

	setLogger(cli.Flag("debug"))
	terraformPath, pathIncludeRegexp, pathExcludeRegexp, typeRegexp, nameRegexp,
		displayAttributes, outputFile, overridesPath, cachePath, onlyRoot, includeFilenames,
		minify, module := getGenArgsAndFlags(cli)

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

		err = container.WalkOverrides(overrides, traverser)
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

//
//nolint:goconst
func getGenArgsAndFlags(
	cli *broccli.Broccli,
) (string, string, string, string, string, string, string, string, string, bool, bool, bool, bool) {
	terraformPath := cli.Arg("path")
	outputFile := cli.Arg("output")
	pathIncludeRegexp := cli.Flag("path-include-regexp")
	pathExcludeRegexp := cli.Flag("path-exclude-regexp")
	typeRegexp := cli.Flag("type-regexp")
	nameRegexp := cli.Flag("name-regexp")
	overrides := cli.Flag("overrides")
	onlyRoot := cli.Flag("only-root")
	includeFilenames := cli.Flag("include-filenames")
	minify := cli.Flag("minify")
	module := cli.Flag("module")
	displayAttributes := cli.Flag("display-attributes")
	cache := cli.Flag("cache")

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
	slog.Info("✨ External modules overrides file: " + overrides)
	slog.Info("✨ Draw only root path:             " + onlyRoot)
	slog.Info("✨ Include source filename:         " + includeFilenames)
	slog.Info("✨ Minify element names:            " + minify)
	slog.Info("✨ Draw 'modules' sub-directory:    " + module)
	slog.Info("✨ Cache path:                      " + cache)

	return terraformPath, pathIncludeRegexp, pathExcludeRegexp, typeRegexp, nameRegexp, displayAttributes, outputFile,
		overrides, cache, onlyRoot == "true", includeFilenames == "true", minify == "true", module == "true"
}

func setLogger(debug string) {
	logLevel := slog.LevelInfo
	if debug == "true" {
		logLevel = slog.LevelDebug
	}

	slog.SetLogLoggerLevel(logLevel)
}
