# tfsketch

[![Go Report Card](https://goreportcard.com/badge/github.com/keenbytes/tfsketch)](https://goreportcard.com/report/github.com/keenbytes/tfsketch)

![tfsketch](tfsketch.png "tfsketch")

A lightweight tool that scans Terraform code for a specified resource type (e.g. `aws_iam_role`) and generates a Mermaid flowchart, along with a summary JSON file of the modules found. It supports scanning modules and nested sub-modules, as well as mapping external modules to local paths via a YAML file (e.g. a cloned Git repository). By default, it scans only one level of sub-directories, treating any `modules` directory as containing externally accessible sub-modules.

## Running
Check below help message for `gen` command:

    Usage:  tfsketch gen [FLAGS] DIR RESOURCE_TYPE FILE
    
    Generate diagram
    
    Optional flags: 
      -d,		 --debug  			Debug mode
      -d2,		 --include-filenames  		Put source filenames on the diagram
      -d1,		 --only-root  			Draw only root directory
      -o,		 --overrides FILE 		File with local paths to external modules

The command takes 3 arguments:

* `DIR` is the path with Terraform code to scan
* `RESOURCE_TYPE` is type of the resource to search for, eg. `aws_iam_role`
* `FILE` is a name of output Mermaid chart file

## Examples
Navigate to `tests` directory for samples.
