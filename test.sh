#!/bin/bash
crd_api_array=()
files=$(git status --porcelain | egrep "apis|config" | cut -b4-)
for file in $files; do
    crd_api_array+=($file)
done
# for each in "${crd_api_array[@]}"
# do
#   echo "$each"
# done
# #echo $crd_api_array