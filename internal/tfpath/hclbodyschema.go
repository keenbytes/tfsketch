package tfpath

import "github.com/hashicorp/hcl/v2"

// NewHCLBodySchema returns new HCL body schema that defines attributes which are meant to be parsed.
func NewHCLBodySchema(displayAttributes []string) *hcl.BodySchema {
	hclBodySchema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{},
	}

	// Resource attributes
	for _, attribute := range displayAttributes {
		if attribute == "" {
			continue
		}

		hclBodySchema.Attributes = append(hclBodySchema.Attributes, hcl.AttributeSchema{
			Name:     attribute,
			Required: false,
		})
	}

	// Other attributes (for modules and syntax)
	for _, attribute := range []string{"source", "version", "for_each"} {
		hclBodySchema.Attributes = append(hclBodySchema.Attributes, hcl.AttributeSchema{
			Name:     attribute,
			Required: false,
		})
	}

	return hclBodySchema
}
