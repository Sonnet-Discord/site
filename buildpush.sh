#!/bin/bash
./gensite
git add -A
git commit -m "${1}"
git push
git subtree push --prefix html origin gh-pages
