package main

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

var (
	errLinkExternalModules = errors.New("error linking external modules")
)

func errLinkExternalModulesWithPath(path string) error {
	return fmt.Errorf("%w: %s", errLinkExternalModules, path)
}

func linkExternalModulesInTerraformDirectory(tfPath *tfPath, moduleKey string, iteration int) (bool, error) {
	externalModulesMissing, err := linkExternalModules(tfPath, moduleKey, iteration)
	if err != nil {
		return false, fmt.Errorf("%w: %w", errLinkExternalModulesWithPath(tfPath.path), err)
	}

	pathsSorted := tfPath.tfPathsSorted()
	for _, pathKey := range pathsSorted {
		subTfPath := tfPath.tfPaths[pathKey]
		if subTfPath == nil {
			continue
		}

		_, isModuleDir := tfPath.tfPathsModules[pathKey]
		if isModuleDir {
			continue
		}

		if len(subTfPath.modules) == 0 {
			continue
		}

		externalModuleMissingSubTfPath, err := linkExternalModules(subTfPath, moduleKey, iteration)
		if err != nil {
			return false, fmt.Errorf("%w: %w", errLinkExternalModulesWithPath(subTfPath.path), err)
		}

		if externalModuleMissingSubTfPath {
			externalModulesMissing = true
		}
	}

	return externalModulesMissing, nil
}

func linkExternalModules(tfPath *tfPath, moduleKey string, iteration int) (bool, error) {
	externalModulesMissing := false

	modulesSorted := tfPath.modulesSorted()
	for _, moduleSource := range modulesSorted {
		moduleSourceArray := strings.Split(moduleSource, ":")
		if len(moduleSourceArray) != 2 {
			continue
		}

		modulePath := moduleSourceArray[1]
		if strings.HasPrefix(modulePath, "./modules") {
			continue
		}

		moduleKeyToSearch := modulePath
		if strings.HasPrefix(modulePath, ".") {
			moduleKeyToSearch = tfPath.moduleSource + "|" + strings.Replace(modulePath, "@", "", 1)
		}

		if tfPath.modules[moduleSource].tfPath != nil {
			continue
		}

		if moduleKeyToSearch == moduleKey {
			continue
		}

		// checking if external modules exists and can be used to link
		moduleTfPath, ok := allTfPaths[moduleKeyToSearch]
		if !ok {
			externalModulesMissing = true

			slog.Error(
				"external module not found in allTfPaths",
				slog.String("module", moduleKeyToSearch),
				slog.String("path", tfPath.modules[moduleSource].filePath),
				slog.Int("iteration", iteration),
			)

			continue
		}

		tfPath.modules[moduleSource].tfPath = moduleTfPath
		slog.Debug(
			"got module link to external module",
			slog.String("module_key", moduleKeyToSearch),
			slog.String("module_path", moduleTfPath.path),
			slog.String("path", tfPath.path),
			slog.String("path_module_name", moduleSourceArray[0]),
			slog.String("path_module_path", modulePath),
			slog.Int("resource_num", len(moduleTfPath.resources)),
			slog.Int("iteration", iteration),
		)
	}

	return externalModulesMissing, nil
}
