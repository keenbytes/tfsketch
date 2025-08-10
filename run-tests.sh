#!/bin/bash

go build .

./tfsketch gen -t '^type$' tests/01-only-resources/ tests/01-only-resources.mmd
mmdc -i tests/01-only-resources.mmd -o tests/01-only-resources.svg --configFile=tests/config.json

./tfsketch gen -t '^type$' tests/02-local-modules/ tests/02-local-modules.mmd
mmdc -i tests/02-local-modules.mmd -o tests/02-local-modules.svg --configFile=tests/config.json

./tfsketch gen -t '^nevermind|type$' -d3 -o tests/external-modules.yml tests/03-external-modules/ tests/03-external-modules.mmd
mmdc -i tests/03-external-modules.mmd -o tests/03-external-modules.svg --configFile=tests/config.json
