# tfsketch

[![Go Reference](https://pkg.go.dev/badge/gopkg.pl/keenbytes/tfsketch.svg)](https://pkg.go.dev/gopkg.pl/keenbytes/tfsketch) [![Go Report Card](https://goreportcard.com/badge/gopkg.pl/keenbytes/tfsketch)](https://goreportcard.com/report/gopkg.pl/keenbytes/tfsketch)

![tfsketch](tfsketch.png "tfsketch")

A tool for generating clean, minimal SVG diagrams from Terraform modules.  
It’s still under active development — new version and documentation are coming soon.

## Building
Run `go build -o tfsketch` to compile the binary.

## Running
Check below help message for `module` command:

    Usage:  tfsketch module [FLAGS] MODULE_DIR OUTPUT_FILE

`MODULE_DIR` is a path to directory containing Terraform module, and `OUTPUT_FILE` is path to a file where output SVG should be written, eg. `./diagram.svg`.

## TODO
- [ ] Add `module` blocks
- [ ] Add `provider` blocks
- [ ] Introduce a config file
- [ ] Allow showing only selected resources
- [ ] Allow replacing resources with custom items or text
- [ ] Distinguish optional and required inputs; display required ones first
- [ ] On item mouseover, display its links to other items
- [ ] Add examples
