package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

type TfVariable struct {
	Name string
	Description string
	Default *string
}

type TfOutput struct {
	Name string
	Description string
}

type TfResource struct {
	Name string
	Kind string
}

type TfModule struct {
	Variables map[string]*TfVariable
	Outputs map[string]*TfOutput
	Resources map[string]*TfResource
}

func parseTerraformModule(dir string) (*TfModule, error) {
	parser := hclparse.NewParser()
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	module := &TfModule{
		Variables: make(map[string]*TfVariable, 0),
		Outputs: make(map[string]*TfOutput, 0),
		Resources: make(map[string]*TfResource, 0),
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".tf") {
			continue
		}

		fullPath := filepath.Join(dir, f.Name())
		content, err := getFileBlocks(parser, fullPath)
		if err != nil {
			return nil, fmt.Errorf("error with file %s: %w", fullPath, err)
		}

		for _, block := range content.Blocks {
			name := block.Labels[0]

			switch block.Type {
			case "variable":
				tfVar, err := getVariableBlock(block)
				if err != nil {
					return nil, fmt.Errorf("error getting variable in file %s: %w", fullPath, err)
				}
				module.Variables[name] = tfVar
			case "output":
				tfOut, err := getOutputBlock(block)
				if err != nil {
					return nil, fmt.Errorf("error getting output in file %s: %w", fullPath, err)
				}
				module.Outputs[name] = tfOut
			case "resource":
				resourceKind := block.Labels[0]
				resourceName := block.Labels[1]
				module.Resources[resourceName] = &TfResource{
					Name: resourceName,
					Kind: resourceKind,
				}
			}
		}
	}

	return module, nil
}

func getFileBlocks(parser *hclparse.Parser, filePath string) (*hcl.BodyContent, error) {
	src, diags := parser.ParseHCLFile(filePath)
	if diags.HasErrors() {
		return nil, fmt.Errorf("error parsing: %s", diags.Error())
	}

	content, _, diags := src.Body.PartialContent(TfBodySchema)
	if diags.HasErrors() {
		return nil, fmt.Errorf("error getting content: %s", diags.Error())
	}

	return content, nil
}

func getVariableBlock(block *hcl.Block) (*TfVariable, error) {
	name := block.Labels[0]

	bodyContent, diags := block.Body.Content(TfVariableSchema)
	if diags.HasErrors() {
		return nil, fmt.Errorf("error parsing block %s.%s: %s", block.Type, name, diags.Error())
	}

	tfVar := &TfVariable{
		Name: name,
	}

	for key, val := range bodyContent.Attributes {
		switch key {
		case "description":
			valExpr, _ := val.Expr.Value(&hcl.EvalContext{})
			tfVar.Description = valExpr.AsString()
		case "default":
			valExpr, _ := val.Expr.Value(&hcl.EvalContext{})
			valWithCty := valExpr.GoString()[4:]
			tfVar.Default = &valWithCty
		}
	}
	
	return tfVar, nil
}

func getOutputBlock(block *hcl.Block) (*TfOutput, error) {
	name := block.Labels[0]

	bodyContent, diags := block.Body.Content(TfOutputSchema)
	if diags.HasErrors() {
		return nil, fmt.Errorf("error parsing block %s.%s: %s", block.Type, name, diags.Error())
	}

	tfOut := &TfOutput{
		Name: name,
	}

	for key, val := range bodyContent.Attributes {
		switch key {
		case "description":
			valExpr, _ := val.Expr.Value(&hcl.EvalContext{})
			tfOut.Description = valExpr.AsString()
		}
	}

	return tfOut, nil
}
