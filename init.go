package main

import hcl "github.com/hashicorp/hcl/v2"

var TfVariableSchema *hcl.BodySchema
var TfOutputSchema *hcl.BodySchema
var TfBodySchema *hcl.BodySchema

func init() {
	TfVariableSchema = &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "type",
				Required: false,
			},
			{
				Name: "default",
				Required: false,
			},
			{
				Name: "required",
				Required: false,
			},
			{
				Name: "description",
				Required: false,
			},
			{
				Name: "validation",
				Required: false,
			},
		},
	}

	TfOutputSchema = &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "value",
				Required: false,
			},
			{
				Name: "description",
				Required: false,
			},
		},
	}

	TfBodySchema = &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "variable", LabelNames: []string{"name"}},
			{Type: "output", LabelNames: []string{"name"}},
			{Type: "resource", LabelNames: []string{"kind", "name"}},
		},
	}
}
