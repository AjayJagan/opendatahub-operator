#!/bin/bash
# crd_api_array=()
# files=$(git status --porcelain | egrep "apis|config" | cut -b4-)
# for file in $files; do
#     echo $file
#     crd_api_array+=($file)
# done
# printf '%s\n' "${my_array[@]}"
#echo $crd_api_array

if [[ -n $(git status -s) ]]
then
    echo "crd changed"
fi  