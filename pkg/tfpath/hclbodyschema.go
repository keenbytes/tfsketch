package tfpath

import "github.com/hashicorp/hcl/v2"

// NewHCLBodySchema returns new HCL body schema that defines attributes which are meant to be parsed.
func NewHCLBodySchema() *hcl.BodySchema {
	hclBodySchema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			// Resource attributes
			{
				Name:     "name",
				Required: false,
			},
			{
				Name:     "id",
				Required: false,
			},
			{
				Name:     "name_prefix",
				Required: false,
			},

			// Module attributes
			{
				Name:     "source",
				Required: false,
			},
			{
				Name:     "version",
				Required: false,
			},
			{
				Name:     "for_each",
				Required: false,
			},
		},
	}

	return hclBodySchema
}
