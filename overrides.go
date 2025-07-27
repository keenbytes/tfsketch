package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
)

var (
	ErrRead      = errors.New("error reading file")
	ErrUnmarshal = errors.New("error unmarshaling yaml file")
)

// ExternalModule represents an external module from the registry.
type ExternalModule struct {
	// Source is in the format of "domain.name@version". Optionally, it can have a ":/../subpath".
	Source string `yaml:"source"`

	// LocalPath is a directory on the local system where application will look for the module source code.
	LocalPath string `yaml:"localPath"`
}

// Overrides represents a YAML file that contains local paths where external Terraform module are meant to be found.
// Application cannot get the source of a module from Terraform registry yet. Hence, it needs to be put locally.
type Overrides struct {
	ExternalModules []*ExternalModule `yaml:"externalModules"`
}

// ReadFromFile takes a YAML file and gets its entries.
func (o *Overrides) ReadFromFile(path string) error {
	fileContents, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRead, err)
	}

	err = yaml.Unmarshal(fileContents, &o)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnmarshal, err)
	}

	return nil
}
