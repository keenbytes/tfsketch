package tfpath

// TfResource represents a Terraform resource.
type TfResource struct {
	Type         string
	Name         string
	FileName     string
	FilePath     string
	FieldName    string
	FieldForEach string
}
