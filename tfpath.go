package main

// TfPath represents a path that contains terraform code.
type TfPath struct {
	// Path is the full path.
	Path string

	// Dirs contains directories found in the path.
	Dirs map[string]*Directory

	// DirsModules contains directories found in the path which are modules.
	DirModules map[string]*Directory

	// ModuleSource is the source of the module (as in Override.Source) if the TfPath is a path to terraform module.
	ModuleSource string
}
