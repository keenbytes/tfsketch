package tfpath

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/keenbytes/tfsketch/internal/overrides"
)

var (
	ErrWalkingOverrides      = errors.New("error walking overrides")
	ErrParsingContainerPaths = errors.New("error parsing container paths")
	ErrLinkingContainerPaths = errors.New("error linking container paths")
)

// Container contains TfPath instance for external modules and the root of the local path that is scanned.
type Container struct {
	Paths map[string]*TfPath
}

// NewContainer returns a new Container.
func NewContainer() *Container {
	container := &Container{
		Paths: map[string]*TfPath{},
	}

	return container
}

// AddPath adds a new TfPath to the container (external module).
func (c *Container) AddPath(name string, tfPath *TfPath) {
	c.Paths[name] = tfPath

	slog.Info(fmt.Sprintf("🔸 Module added: 📦%s in 📁%s", name, tfPath.Path))
}

// WalkOverrides runs traverser's WalkPath on each entry from Override object
func (c *Container) WalkOverrides(overrides *overrides.Overrides, traverser *Traverser) error {
	externalModulesNum := len(overrides.ExternalModules)
	slog.Info(
		fmt.Sprintf("🔸 External modules number in overrides file: %d", externalModulesNum),
	)
	
	for _, externalModule := range overrides.ExternalModules {
		tfPath := NewTfPath(externalModule.Local, externalModule.Remote)
		c.AddPath(tfPath.TraverseName, tfPath)

		isSubModule := c.isExternalModuleASubModule(externalModule.Remote)

		err := traverser.WalkPath(tfPath, !isSubModule)
		if err != nil {
			slog.Error(
				fmt.Sprintf(
					"❌ Error walking dirs in overrides local path 📁%s: %s",
					externalModule.Local,
					err.Error(),
				),
			)

			return fmt.Errorf("%w: %w", ErrWalkingOverrides, err)
		}
	}

	return nil
}

// ParsePaths runs traverser's ParsePath on each path
func (c *Container) ParsePaths(traverser *Traverser) error {
	for pathName, tfPath := range c.Paths {
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

			return fmt.Errorf("%w: %w", ErrParsingContainerPaths, err)
		}
	}

	return nil
}

// LinkPaths will run traverser's LinkPath on each path
func (c *Container) LinkPaths(traverser *Traverser) error {
	for pathName, tfPath := range c.Paths {
		err := traverser.LinkPath(tfPath)
		if err != nil {
			slog.Error(
				fmt.Sprintf(
					"❌ Error linking local modules in terraform path 📁%s (%s) : %s",
					tfPath.Path,
					pathName,
					err.Error(),
				),
			)

			return fmt.Errorf("%w: %w", ErrLinkingContainerPaths, err)
		}
	}

	return nil
}

func (c *Container) isExternalModuleASubModule(module string) bool {
	return strings.Contains(module, "//modules/")
}
