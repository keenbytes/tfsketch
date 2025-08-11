# tfsketch

[![Go Report Card](https://goreportcard.com/badge/github.com/keenbytes/tfsketch)](https://goreportcard.com/report/github.com/keenbytes/tfsketch)

![tfsketch](tfsketch.png "tfsketch")

A lightweight tool that scans Terraform code for a specified resource type (e.g. `aws_iam_role`) and generates a Mermaid flowchart, along with a summary JSON file of the modules found. It supports scanning modules and nested sub-modules, as well as mapping external modules to local paths via a YAML file (e.g. a cloned Git repository). By default, it scans only one level of sub-directories, treating any `modules` directory as containing externally accessible sub-modules.

## Preview
Example diagram generated from tests/03-external-modules using tests/external-modules.yaml:

![preview](preview.png "chart preview")

**Prerequisite**: Install [Mermaid CLI](https://github.com/mermaid-js/mermaid-cli) to convert Mermaid diagrams to SVG.

**Commands**:
```
./tfsketch gen -o tests/external-modules.yml -t '^type$' tests/03-external-modules tmp/03-external-modules.mmd
mmdc -i tmp/03-external-modules.mmd -o tmp/03-external-modules.mmd.svg --configFile=tests/config.json
```

**Alternatively with docker**:
```
docker run \
  -w / \
  -v $(pwd)/tests:/tests \
  -v $(pwd)/tmp:/output \
  keenbytes/tfsketch:v0.4.0 \
  gen -o /tests/external-modules.yml -t '^type$' /tests/03-external-modules /output/03-external-modules.mmd
```

Check `tests` directory for more examples.

## Building
Run the following command to compile the binary:
```
go build .
```

## Running
Check below help message for `gen` command:

    Usage:  tfsketch gen [FLAGS] DIR FILE
    
    Generate diagram
    
    Optional flags: 

    -d,  --debug                Enable debug mode
    -f,  --include-filenames    Display source filenames on the diagram
    -s,  --minify               Minify element names in the chart to save space
    -n,  --module               Treat path as module and draw 'modules' sub-directory
    -r,  --only-root            Draw only root directory
    -o,  --overrides FILE       YAML file mapping external modules to local paths
    -t,  --type-regexp REGEXP   Regular expression to filter type of the resource

The command accepts three arguments:

* `DIR` – Path to the Terraform code to scan.
* `FILE` – Output Mermaid chart file name.

### Using docker
A Docker image is also available, though it requires binding local volumes.

```
docker run --platform linux/amd64 keenbytes/tfsketch:v0.4.0 gen -h
```

## Motivation
**tfsketch** began as a small helper tool for navigating repositories packed with complex Terraform code, particularly in cases where specific resources—such as AWS IAM roles—needed to be refactored. It was also designed for situations where multiple repositories were being standardised to follow a consistent structure. By using the tool, it becomes easier to visualise repository contents and analyse their structure.

