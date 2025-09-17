// Package remotetolocal provides a data model for mapping a remote resource
// (identified by a URL or a regular‑expression pattern) to a corresponding
// local filesystem path.
package remotetolocal

// RemoteToLocal defines a single mapping from a remote source to a local
// destination.
type RemoteToLocal struct {
	// Remote specifies the remote identifier. It can be either:
	//
	//   • A concrete URL with an optional version suffix, e.g. "repo.git@v1.2".
	//   • A regular‑expression pattern that must begin with '^'. When a
	//     regexp is used, any captured groups (parenthesized sub‑expressions)
	//     become placeholders that can be referenced in the Local field as
	//     {1}, {2}, … .
	Remote string `yaml:"remote"`

	// Local is the local directory that will hold the fetched remote resource.
	//
	// If Remote is a regular expression, Local acts as a template. The captured
	// groups from Remote can be interpolated using the same {1}, {2}, … syntax.
	// For example, with Remote="^github.com/(.+)/(.+)$" and Local="/src/{1}/{2}",
	// a match for "github.com/foo/bar" would resolve to "/src/foo/bar".
	Local string `yaml:"local,omitempty"`

	// Cache is a source that requires downloading (for example using git) to
	// cache directory.  Cache has precedence over Local.
	Cache string `yaml:"cache,omitempty"`
}
