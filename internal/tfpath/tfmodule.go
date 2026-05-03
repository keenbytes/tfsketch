package tfpath

// TfModule represents a reference to a module ('module' resource in Terraform).
type TfModule struct {
	Name         string
	FileName     string
	FilePath     string
	FieldSource  string
	FieldVersion string
	FieldForEach string
	TfPath       *TfPath
}
