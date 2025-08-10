package tfpath

import (
	"fmt"
	"log/slog"
)

// Container contains paths to external modules.
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

// AddPath adds a new path to the container.
func (c *Container) AddPath(name string, tfPath *TfPath) {
	c.Paths[name] = tfPath

	slog.Info(fmt.Sprintf("🔸 Module added: 📦%s in 📁%s", name, tfPath.Path))
}
