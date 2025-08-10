// Package remotetolocal contains a struct used to define mapping of a remote source to local path.
package remotetolocal

// RemoteToLocal represents a mapping from a remote resource to a local one.
type RemoteToLocal struct {
	// Remote is any remote URL in format url@version[|internal_path]
	Remote string `yaml:"remote"`

	// Local is a local directory containing the remote resource.
	Local string `yaml:"local"`
}
