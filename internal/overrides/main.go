// Package overrides contains struct for overrides file.
package overrides

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/keenbytes/tfsketch/internal/remotetolocal"
	yaml "gopkg.in/yaml.v2"
)

// Overrides represents a YAML file that contains local paths where external Terraform module are meant to be found.
// Application cannot get the source of a module from Terraform registry yet. Hence, it needs to be put locally.
type Overrides struct {
	ExternalModules []*remotetolocal.RemoteToLocal `yaml:"externalModules"`
}

var (
	ErrRead         = errors.New("error reading file")
	ErrUnmarshal    = errors.New("error unmarshaling yaml file")
)

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

// Reset removes all attached external modules
func (o *Overrides) Reset() {
	o.ExternalModules = []*remotetolocal.RemoteToLocal{}
}

// AddExternalModule adds an externalmodule
func (o *Overrides) AddExternalModule(remote, local string) {
	o.ExternalModules = append(o.ExternalModules, &remotetolocal.RemoteToLocal{
		Remote: remote,
		Local: local,
	})
}
