package main

import (
	"errors"
	"fmt"
	"log/slog"
)

var (
	errTfPathWalkDir           = errors.New("error walking through tf path")
	errTfPathParse             = errors.New("error parsing tf path files")
	errTfPathLinkSubdirModules = errors.New("error linking modules in sub-directories")
)

func traverseTerraformDirectory(path string, moduleSource string, resourceType string) error {
	tfPathInstance := tfPath{
		path:           path,
		tfPaths:        map[string]*tfPath{},
		tfPathsModules: map[string]struct{}{},
		resources:      map[string]*resource{},
		modules:        map[string]*module{},
		moduleSource:   moduleSource,
	}

	err := tfPathInstance.walkDir()
	if err != nil {
		return fmt.Errorf("%w: %w", errTfPathWalkDir, err)
	}

	err = tfPathInstance.parse(resourceType)
	if err != nil {
		return fmt.Errorf("%w: %w", errTfPathParse, err)
	}

	err = tfPathInstance.linkModulesInSubdirectories()
	if err != nil {
		return fmt.Errorf("%w: %w", errTfPathLinkSubdirModules, err)
	}

	allTfPaths[moduleSource] = &tfPathInstance
	slog.Debug(
		"added global path",
		slog.String("key", moduleSource),
		slog.String("path", tfPathInstance.path),
	)

	return nil
}
