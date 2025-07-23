package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/keenbytes/broccli/v3"
	yaml "gopkg.in/yaml.v2"
)

var (
	errTerraformTraverse = errors.New("error traversing dir with tf code")
)

func errorTraversingTerraformDir(err error) error {
	return fmt.Errorf("%w: %s", errTerraformTraverse, err.Error())
}

type Overrides struct {
	ExternalModules []*ExternalModule `yaml:"externalModules"`
}

type ExternalModule struct {
	Source string `yaml:"source"`
	Version string `yaml:"version"`
	ModulePath string `yaml:"modulePath,omitempty"`
	LocalPath string `yaml:"localPath"`
}

func main() {
	cli := broccli.NewBroccli("tfsketch", "Generate diagram from Terraform files", "Mikolaj Gasior <m@gasior.dev>")

	cmd := cli.Command("gen", "Generate diagram", genHandler)
	cmd.Arg("path", "DIR", "Path to directory with terraform code", broccli.TypePathFile, broccli.IsDirectory|broccli.IsExistent|broccli.IsRequired)
	cmd.Arg("type", "RESOURCE_TYPE", "Type of the resource to search for", broccli.TypeString, broccli.IsRequired)
	cmd.Flag("overrides", "o", "FILE", "File with local paths to external modules", broccli.TypePathFile, broccli.IsRegularFile)
	cmd.Flag("debug", "d", "", "Debug mode", broccli.TypeBool, 0)

	os.Exit(cli.Run(context.Background()))
}

var allDirs = map[string]*DirContainer{}

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
	if overridesPath != "" {
		overrides = getOverrides(overridesPath)
		for _, externalModule := range overrides.ExternalModules {
			externalModuleKey := externalModule.Source+"@"+externalModule.Version
			if externalModule.ModulePath != "" {
				externalModuleKey += "|" + externalModule.ModulePath
			}
			err := traverseTerraformDirectory(externalModule.LocalPath, externalModuleKey, resourceType)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error traversing override external module in local path %s: %s", externalModule.LocalPath, errorTraversingTerraformDir(err))
				return 2
			}
		}
	}

	err := traverseTerraformDirectory(terraformDir, ".", resourceType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error traversing root terraform dir: %s", errorTraversingTerraformDir(err))
		return 1
	}

	fmt.Fprintf(os.Stdout, "\n\nExternal tf modules placed locally (from overrides):\n")
	for key, dir := range allDirs {
		if key == "." {
			continue
		}
		fmt.Fprintf(os.Stdout, "%s -> %s\n", key, dir.Root)
	}

	// generate for the root dir
	genMermaid(allDirs["."].Dirs)

	return 0
}

func getOverrides(path string) *Overrides {
	fileContents, err := os.ReadFile(path)
	if err != nil {
		log.Fatal("error reading the overrides file")
	}

	var overridesYaml *Overrides
	err = yaml.Unmarshal(fileContents, &overridesYaml)
	if err != nil {
		log.Fatal("error unmarshalling overrides file")
	}

	return overridesYaml
}
