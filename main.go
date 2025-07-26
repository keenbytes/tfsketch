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
	errOverrideTraverse = errors.New("error traversing overrides")
)

var allDirs = map[string]*DirContainer{}

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

	err := traverseOverrides(cli.Flag("overrides"), resourceType)
	if err != nil {
		return 3
	}

	err = traverseTerraformDirectory(terraformDir, ".", resourceType)
	if err != nil {
		slog.Error(
			"error traversing root terraform dir",
			slog.String("path", terraformDir),
			slog.String("resourceType", resourceType),
			slog.String("error", errorTraversingTerraformDir(err).Error()),
		)
		return 1
	}

	for key, dir := range allDirs {
		if key == "." {
			continue
		}
		slog.Info(
			"got external module",
			slog.String("module", key),
			slog.String("path", dir.Root),
		)
	}

	// generate for the root dir
	genMermaid(allDirs["."].Dirs, resourceType, outputFile)

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

func traverseOverrides(path string, resourceType string) error {
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

			for dirKey, dir := range allDirs[externalModule.Source].Dirs {
				slog.Debug(
					"got directory from traversing an override",
					slog.String("module", externalModule.Source),
					slog.String("directory", dir.FullPath),
					slog.String("key", dirKey),
				)
			}

			for dirKey, dirModule := range allDirs[externalModule.Source].DirsModules {
				slog.Debug(
					"got module directory from traversing an override",
					slog.String("module", externalModule.Source),
					slog.String("directory", dirModule.FullPath),
					slog.String("key", dirKey),
				)
			}
		}

		slog.Debug("finished traversing overrides")
	} else {
		slog.Debug("no overrides")
	}

	return nil
}
