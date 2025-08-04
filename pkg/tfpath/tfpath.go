package tfpath

import "sort"

// TfPath represents a path that contains terraform code.
type TfPath struct {
	// Path is the full path.
	Path string

	// TraverseName is the source of the module (as in Override.Source) if the tfPath is a path to terraform module.
	TraverseName string

	// RelPath is a relative path - hence does not contain the parent/base.
	RelPath string

	// Children contains directories found in the path.
	Children map[string]*TfPath

	// IsChildModule contains map with names of paths that are modules.
	IsChildModule map[string]struct{}

	// resources contains tf resources found in the code
	Resources map[string]*TfResource

	// modules contains tf modules found in the code
	Modules map[string]*TfModule
}

// NewTfPath returns new TfPath instance containing name and a path.
func NewTfPath(path string, name string) *TfPath {
	tfPath := &TfPath{
		Path: path,
		TraverseName: name,
		Children:        map[string]*TfPath{},
		IsChildModule: map[string]struct{}{},
		Resources:      map[string]*TfResource{},
		Modules:        map[string]*TfModule{},
	}

	return tfPath
}

func (t *TfPath) ChildrenNamesSorted() []string {
	namesSorted := make([]string, len(t.Children))
	for childKey, _ := range t.Children {
		namesSorted = append(namesSorted, childKey)
	}
	sort.Strings(namesSorted)

	return namesSorted
}

func (t *TfPath) ResourceNamesSorted() []string {
	namesSorted := make([]string, len(t.Children))
	for resourceKey, _ := range t.Resources {
		namesSorted = append(namesSorted, resourceKey)
	}
	sort.Strings(namesSorted)

	return namesSorted
}

func (t *TfPath) ModuleNamesSorted() []string {
	namesSorted := make([]string, len(t.Children))
	for moduleKey, _ := range t.Modules {
		namesSorted = append(namesSorted, moduleKey)
	}
	sort.Strings(namesSorted)

	return namesSorted
}
