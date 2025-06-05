package main

import (
	"context"
	"fmt"
	"os"

	"github.com/keenbytes/broccli/v3"
)

func main() {
	cli := broccli.NewBroccli("tfdiagram", "Generate SVG from terraform files", "Mikolaj Gasior <m@gasior.dev>")

	cmd := cli.Command("module", "Generate an SVG diagram from a terraform module", drawHandler)
	cmd.Arg("module-dir", "MODULE_DIR", "Path to terraform module", broccli.TypePathFile, broccli.IsDirectory|broccli.IsExistent|broccli.IsRequired)
	cmd.Arg("output-file", "OUTPUT_FILE", "Path to output SVG file", broccli.TypePathFile, broccli.IsRequired)

	_ = cli.Command("version", "Prints version", versionHandler)
	if len(os.Args) == 2 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		os.Args = []string{"App", "version"}
	}

	os.Exit(cli.Run(context.Background()))
}

func drawHandler(ctx context.Context, c *broccli.Broccli) int {
	moduleDir := c.Arg("module-dir")
	outputFile := c.Arg("output-file")

	module, err := parseTerraformModule(moduleDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse module: %v\n", err)
		return 1
	}

	svg := generateSvg(module)
	os.WriteFile(outputFile, []byte(svg), 0644)

	return 0
}

func versionHandler(ctx context.Context, c *broccli.Broccli) int {
	fmt.Fprintf(os.Stdout, VERSION+"\n")
	return 0
}
