package tfpath

import (
	"fmt"
	"log/slog"
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
