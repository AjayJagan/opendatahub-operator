#!/bin/sh

files=$(git status --porcelain | cut -b4-)
for file in $files; do
    echo $file
done