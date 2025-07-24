package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/keenbytes/broccli/v3"
	yaml "gopkg.in/yaml.v2"
)


func main() {
	cli := broccli.NewBroccli("tfsketch", "Generate diagram from Terraform files", "Mikolaj Gasior <m@gasior.dev>")

	cmd := cli.Command("gen", "Generate diagram", genHandler)
	cmd.Arg("path", "DIR", "Path to directory with terraform code", broccli.TypePathFile, broccli.IsDirectory|broccli.IsExistent|broccli.IsRequired)
	cmd.Arg("type", "RESOURCE_TYPE", "Type of the resource to search for", broccli.TypeString, broccli.IsRequired)
	cmd.Flag("overrides", "o", "FILE", "File with local paths to external modules", broccli.TypePathFile, broccli.IsRegularFile)
	cmd.Flag("debug", "d", "", "Debug mode", broccli.TypeBool, 0)

	os.Exit(cli.Run(context.Background()))
}

func getModulePathFromExternalModuleSource(source string) string {
	arr := strings.Split(source, "|")
	if len(arr) == 2 {
		return arr[1]
	}

	return ""
}

func getSourceFromExternalModuleSource(source string) string {
	arr := strings.Split(source, "|")
	if len(arr) == 2 {
		source = arr[0]
	}

	arr = strings.Split(source, "@")
	if len(arr) == 2 {
		return arr[0]
	}

	return ""
}

func getVersionFromExternalModuleSource(source string) string {
	arr := strings.Split(source, "|")
	if len(arr) == 2 {
		source = arr[0]
	}

	arr = strings.Split(source, "@")
	if len(arr) == 2 {
		return arr[1]
	}

	return ""
}

func genHandler(_ context.Context, cli *broccli.Broccli) int {
	logLevel := slog.LevelInfo
	debug := cli.Flag("debug")
	if debug == "true" {
		logLevel = slog.LevelDebug
	}
	
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, opts))
	slog.SetDefault(logger)

	terraformDir := cli.Arg("path")
	resourceType := cli.Arg("type")

	overridesPath := cli.Flag("overrides")

	var overrides *Overrides
	var err error
	if overridesPath != "" {
		overrides, err = getOverrides(overridesPath)
		if err != nil {
			slog.Error(
					"Error getting overrides",
					slog.String("path", overridesPath),
					slog.String("error", errorGettingOverrides(err).Error()),
				)
				return 3
		}

		for _, externalModule := range overrides.ExternalModules {
			err = traverseTerraformDirectory(externalModule.LocalPath, externalModule.Source, resourceType)
			if err != nil {
				slog.Error(
					"Error traversing override external module in local path",
					slog.String("path", externalModule.LocalPath),
					slog.String("error", errorTraversingTerraformDir(err).Error()),
				)
				return 2
			}
		}
	}

	err = traverseTerraformDirectory(terraformDir, ".", resourceType)
	if err != nil {
		slog.Error(
			"Error traversing root terraform dir",
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
			"Got external module:",
			slog.String("module", key),
			slog.String("path", dir.Root),
		)
	}

	// generate for the root dir
	genMermaid(allDirs["."].Dirs, resourceType)

	return 0
}

func getOverrides(path string) (*Overrides, error) {
	fileContents, err := os.ReadFile(path)
	if err != nil {
		return nil, errOverridesRead
	}

	var overridesYaml *Overrides
	err = yaml.Unmarshal(fileContents, &overridesYaml)
	if err != nil {
		return nil, errOverridesUnmarshal
	}

	return overridesYaml, nil
}
