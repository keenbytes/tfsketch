package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/keenbytes/broccli/v3"
)

var (
	errTerraformTraverse = errors.New("error traversing dir with tf code")
)

func errorTraversingTerraformDir(err error) error {
	return fmt.Errorf("%w: %s", errTerraformTraverse, err.Error())
}

func main() {
	cli := broccli.NewBroccli("tfsketch", "Generate diagram from Terraform files", "Mikolaj Gasior <m@gasior.dev>")

	cmd := cli.Command("gen", "Generate diagram", genHandler)
	cmd.Arg("path", "DIR", "Path to directory with terraform code", broccli.TypePathFile, broccli.IsDirectory|broccli.IsExistent|broccli.IsRequired)
	cmd.Arg("type", "RESOURCE_TYPE", "Type of the resource to search for", broccli.TypeString, broccli.IsRequired)

	_ = cli.Command("version", "Prints version", versionHandler)
	if len(os.Args) == 2 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		os.Args = []string{"App", "version"}
	}

	os.Exit(cli.Run(context.Background()))
}

func genHandler(_ context.Context, cli *broccli.Broccli) int {
	terraformDir := cli.Arg("path")
	resourceType := cli.Arg("type")

	err := traverseTerraformDirectory(terraformDir, resourceType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", errorTraversingTerraformDir(err))
		return 1
	}

	return 0
}

func versionHandler(_ context.Context, _ *broccli.Broccli) int {
	fmt.Fprintf(os.Stdout, VERSION+"\n")
	return 0
}
