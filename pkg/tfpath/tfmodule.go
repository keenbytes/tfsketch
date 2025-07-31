package tfpath

type TfModule struct {
	Name         string
	FileName     string
	FilePath     string
	FieldSource  string
	FieldVersion string
	FieldForEach string
	TfPath       *TfPath
}
