#!/bin/bash

go build .

./tfsketch gen -d -i '^(\.|s.*)$' -e '.*skip.*' -t '^type$' tests/01-only-resources/ tests/01-only-resources.mmd
mmdc -i tests/01-only-resources.mmd -o tests/01-only-resources.svg --configFile=tests/config.json

./tfsketch gen -t '^type$' -a name,id tests/02-local-modules/ tests/02-local-modules.mmd
mmdc -i tests/02-local-modules.mmd -o tests/02-local-modules.svg --configFile=tests/config.json

./tfsketch gen -t '^nevermind|type$' -m -o tests/external-modules.yml tests/03-external-modules/ tests/03-external-modules.mmd
mmdc -i tests/03-external-modules.mmd -o tests/03-external-modules.svg --configFile=tests/config.json

./tfsketch gen -t '^type$' -a name,id tests/04-cache/ tests/04-cache.mmd
mmdc -i tests/04-cache.mmd -o tests/04-cache.svg --configFile=tests/config.json
