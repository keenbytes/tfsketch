package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

var (
	errBodyPartialContent = errors.New("error getting Partial Content from body")
)

var (
	tfResourceDefinition = &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
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
			{
				Name:     "for_each",
				Required: false,
			},
		},
	}
	tfModuleDefinition = &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "source",
				Required: false,
			},
			{
				Name:     "version",
				Required: false,
			},
		},
	}
)

func getNameFromHCLBlock(block *hcl.Block) (string, error) {
	name := block.Labels[0]

	bodyContent, _, diags := block.Body.PartialContent(tfResourceDefinition)
	if diags.HasErrors() {
		return "", fmt.Errorf("%w: %s.%s: %s", errBodyPartialContent, block.Type, name, diags.Error())
	}

	attrToGet := ""
	for attrName, _ := range bodyContent.Attributes {
		if attrName == "name" {
			attrToGet = attrName
			break
		}
		if attrToGet == "" && attrName == "name_prefix" {
			attrToGet = attrName
			break
		}
		if attrToGet == "" && attrName == "id" {
			attrToGet = attrName
			break
		}
	}

	if attrToGet == "" {
		return "no-name-attr", nil
	}

	nameField := ""

	for attrName, attr := range bodyContent.Attributes {
		if attrName != attrToGet {
			continue
		}

		var srcRange hcl.Range
		var found bool

		expr, ok := attr.Expr.(*hclsyntax.TemplateExpr)
		if ok {
			srcRange = expr.SrcRange
			found = true
		}

		if !found {
			scopeTraversalExpr, ok := attr.Expr.(*hclsyntax.ScopeTraversalExpr)
			if ok {
				srcRange = scopeTraversalExpr.SrcRange
				found = true
			}
		}

		if found {
			source, err := os.ReadFile(srcRange.Filename)
			if err == nil {
				raw := string(source[srcRange.Start.Byte:srcRange.End.Byte])
				nameField = raw
			}
		}
	}

	return nameField, nil
}

func getForEachFromHCLBlock(block *hcl.Block) (string, error) {
	name := block.Labels[0]

	bodyContent, _, diags := block.Body.PartialContent(tfResourceDefinition)
	if diags.HasErrors() {
		return "", fmt.Errorf("%w: %s.%s: %s", errBodyPartialContent, block.Type, name, diags.Error())
	}

	forEachField := ""

	for attrName, attr := range bodyContent.Attributes {
		if attrName != "for_each" {
			continue
		}

		var srcRange hcl.Range
		var found bool

		expr, ok := attr.Expr.(*hclsyntax.TupleConsExpr)
		if ok {
			srcRange = expr.SrcRange
			found = true
		}

		if !found {
			scopeTraversalExpr, ok := attr.Expr.(*hclsyntax.ScopeTraversalExpr)
			if ok {
				srcRange = scopeTraversalExpr.SrcRange
				found = true
			}
		}

		if found {
			source, err := os.ReadFile(srcRange.Filename)
			if err == nil {
				raw := string(source[srcRange.Start.Byte:srcRange.End.Byte])
				forEachField = raw
			}
		}
	}

	return forEachField, nil
}

func getSourceFromHCLBlock(block *hcl.Block) (string, string, error) {
	name := block.Labels[0]

	bodyContent, _, diags := block.Body.PartialContent(tfModuleDefinition)
	if diags.HasErrors() {
		return "", "", fmt.Errorf("%w: %s.%s: %s", errBodyPartialContent, block.Type, name, diags.Error())
	}

	fieldValues := make(map[string]string, 2)

	for attrName, attr := range bodyContent.Attributes {
		if attrName != "source" && attrName != "version" {
			continue
		}

		value, _ := attr.Expr.Value(nil)
		if value.Type() == cty.String {
			fieldValues[attrName] = value.AsString()
		}
	}

	return fieldValues["source"], fieldValues["version"], nil
}
