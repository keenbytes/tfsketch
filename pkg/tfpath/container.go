package tfpath

import (
	"fmt"
	"log/slog"
)

type Container struct {
	Paths map[string]*TfPath
}

func NewContainer() *Container {
	container := &Container{
		Paths: map[string]*TfPath{},
	}

	return container
}

func (c *Container) AddPath(name string, tfPath *TfPath) {
	c.Paths[name] = tfPath

	slog.Info(fmt.Sprintf("🔸 Module added: 📦%s in 📁%s", name, tfPath.Path))
}
