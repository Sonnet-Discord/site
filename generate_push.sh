#!/bin/bash
set -euo pipefail
./site_generator/site_generator
git add -A
git commit -m "${1}"
git push
git subtree push --prefix html origin gh-pages
