package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/keenbytes/broccli/v3"
)

var (
	errOverrideReadFromFile = errors.New("error reading overrides from file")
	errOverrideTraverse     = errors.New("error traversing overrides")
	errOverrideLinkModule   = errors.New("error linking override modules")
	errTfDirTraverse        = errors.New("error traversing terraform directory")
	errTfDirLink            = errors.New("error linkin terraform directory modules with external modules")
)

var allTfPaths = map[string]*tfPath{}

const (
	linkModuleIterationsNum = 10
)

func main() {
	cli := broccli.NewBroccli("tfsketch", "Generate diagram from Terraform files", "Mikolaj Gasior <m@gasior.dev>")

	cmd := cli.Command("gen", "Generate diagram", genHandler)
	cmd.Arg("path", "DIR", "Path to directory with terraform code", broccli.TypePathFile, broccli.IsDirectory|broccli.IsExistent|broccli.IsRequired)
	cmd.Arg("type", "RESOURCE_TYPE", "Type of the resource to search for", broccli.TypeString, broccli.IsRequired)
	cmd.Arg("output", "FILE", "Path to an output file", broccli.TypePathFile, broccli.IsRequired)
	cmd.Flag("overrides", "o", "FILE", "File with local paths to external modules", broccli.TypePathFile, broccli.IsRegularFile)
	cmd.Flag("debug", "d", "", "Debug mode", broccli.TypeBool, 0)

	os.Exit(cli.Run(context.Background()))
}

func genHandler(_ context.Context, cli *broccli.Broccli) int {
	setLogger(cli.Flag("debug"))

	terraformDir := cli.Arg("path")
	resourceType := cli.Arg("type")
	outputFile := cli.Arg("output")

	err := traverseAndLinkOverrides(cli.Flag("overrides"), resourceType)
	if err != nil {
		return 3
	}

	err = traverseTerraformDirectory(terraformDir, ".", resourceType)
	if err != nil {
		slog.Error(
			errTfDirTraverse.Error(),
			slog.String("path", terraformDir),
			slog.String("resourceType", resourceType),
			slog.String("error", err.Error()),
		)
		return 2
	}

	_, err = linkExternalModulesInTerraformDirectory(allTfPaths["."], ".", 0)
	if err != nil {
		slog.Error(
			errTfDirLink.Error(),
			slog.String("path", terraformDir),
			slog.String("error", err.Error()),
		)
		return 1
	}

	for key, dir := range allTfPaths {
		if key == "." {
			continue
		}
		slog.Info(
			"got external module",
			slog.String("module", key),
			slog.String("path", dir.path),
		)
	}

	// generate for the root dir
	genMermaid(allTfPaths["."], resourceType, outputFile)

	return 0
}

func setLogger(debug string) {
	logLevel := slog.LevelInfo
	if debug == "true" {
		logLevel = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, opts))
	slog.SetDefault(logger)
}

func traverseAndLinkOverrides(path string, resourceType string) error {
	if path == "" {
		return nil
	}

	overrides := &Overrides{}
	err := overrides.ReadFromFile(path)
	if err != nil {
		slog.Error(
			errOverrideReadFromFile.Error(),
			slog.String("path", path),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("%w: %w", errOverrideReadFromFile, err)
	}

	if len(overrides.ExternalModules) > 0 {
		for _, externalModule := range overrides.ExternalModules {
			err := traverseTerraformDirectory(externalModule.LocalPath, externalModule.Source, resourceType)
			if err != nil {
				slog.Error(
					errOverrideTraverse.Error(),
					slog.String("path", externalModule.LocalPath),
					slog.String("error", err.Error()),
				)
				return fmt.Errorf("%w: %w", errOverrideTraverse, err)
			}

			for dirKey, dir := range allTfPaths[externalModule.Source].tfPaths {
				_, isModule := allTfPaths[externalModule.Source].tfPathsModules[dirKey]
				slog.Debug(
					"got directory from traversing an override",
					slog.String("override", externalModule.Source),
					slog.String("directory", dir.path),
					slog.String("key", dirKey),
					slog.String("is_module", fmt.Sprintf("%v", isModule)),
					slog.Int("resources_num", len(dir.resources)),
				)
			}
		}

		// link modules
		for iteration := 0; iteration < linkModuleIterationsNum; iteration++ {
			continueLinking := false

			for moduleKey, tfPath := range allTfPaths {
				slog.Debug(
					"linking external module to another external module",
					slog.String("module_key", moduleKey),
					slog.String("path", tfPath.path),
					slog.Int("resources_num", len(tfPath.resources)),
					slog.Int("iteration", iteration),
				)
				externalModulesMissing, err := linkExternalModulesInTerraformDirectory(tfPath, moduleKey, iteration)
				if externalModulesMissing {
					continueLinking = true
				}
				if err != nil {
					slog.Error(
						errOverrideLinkModule.Error(),
						slog.String("module_path", tfPath.path),
						slog.String("module_key", moduleKey),
						slog.String("error", err.Error()),
						slog.Int("iteration", iteration),
					)
					return fmt.Errorf("%w: %w", errOverrideTraverse, err)
				}
			}

			if !continueLinking {
				break
			}
		}

		slog.Debug("finished traversing overrides")
	} else {
		slog.Debug("no overrides")
	}

	return nil
}
