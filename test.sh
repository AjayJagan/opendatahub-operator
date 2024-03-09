#!/bin/bash
files=$(git status --porcelain | cut -b4-)
for file in $files; do
    echo $file
    git add $file
    read -p "enter a comment: " comments
    git commit -m "${comments}"
done