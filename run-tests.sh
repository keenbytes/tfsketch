#!/bin/bash

go build .

./tfsketch gen -t '^nevermind$' tests/01-only-resources/ tests/01-only-resources.mmd
mmdc -i tests/01-only-resources.mmd -o tests/01-only-resources.svg

./tfsketch gen -t '^nevermind$' tests/02-local-modules/ tests/02-local-modules.mmd
mmdc -i tests/02-local-modules.mmd -o tests/02-local-modules.svg

./tfsketch gen -t '^nevermind|type$' -o tests/external-modules.yml tests/03-external-modules/ tests/03-external-modules.mmd
mmdc -i tests/03-external-modules.mmd -o tests/03-external-modules.svg
