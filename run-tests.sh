#!/bin/bash

go build .

./tfsketch gen tests/01-only-resources/ type tests/01-only-resources.mmd
mmdc -i tests/01-only-resources.mmd -o tests/01-only-resources.svg

./tfsketch gen tests/02-local-modules/ type tests/02-local-modules.mmd
mmdc -i tests/02-local-modules.mmd -o tests/02-local-modules.svg

./tfsketch gen -o tests/external-modules.yml tests/03-external-modules/ type tests/03-external-modules.mmd
mmdc -i tests/03-external-modules.mmd -o tests/03-external-modules.svg
