package main

import (
	"errors"
	"fmt"
	"regexp"

	hcl "github.com/hashicorp/hcl/v2"
)


var (
	errTerraformTraverse = errors.New("error traversing dir with tf code")
	errOverrideGet = errors.New("error getting overrides")
)

func errorTraversingTerraformDir(err error) error {
	return fmt.Errorf("%w: %s", errTerraformTraverse, err.Error())
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)

var (
	errTfDirWalk = errors.New("error walking tf dir")
)

type Directory struct {
	FullPath    string
	DisplayPath string
	Resources   map[string]*Resource
	Modules map[string]*Directory
	ModulesForEach map[string]string
	ModuleName string
}

type Resource struct {
	Type       string
	Name       string
	FieldName  string
	TfFileName string
	ForEach    string
}

type DirContainer struct {
	Root string
	Dirs map[string]*Directory
	DirsModules map[string]*Directory
}

var (
	TfResourceWithName = &hcl.BodySchema{
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
	TfModule = &hcl.BodySchema{
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

const elementSeparator = "__"
const partSeparator = "_"
