#!/bin/bash
#
# @param $1 - PR number or URL
# wait for a bit until pr is created, otherwise it throws an error "no checks reported on the 'odh-release/e2e-test' branch"
set -euo

sleep 10

pr_has_status() {
    local pr=$1
    local skip=$2
    local status=$3

    gh pr checks $pr | awk -v FS=$'\t'  "\$1 ~ /$skip/{next} \$2 == \"$status\" {found=1} END {if (!found) exit 1}"
}

while pr_has_status $1 tide pending; do
  printf "PR checks still pending, retrying in 10 seconds...\n"
  sleep 30 # replace with 60000
done

if pr_has_status $1 tide fail; then
  printf "!!PR checks failed!!\n"
  exit 1
fi

if pr_has_status $1 tide pass; then
  printf "!!PR checks passed!!\n"
  exit 0
fi

printf "!!An unknown error occurred!!\n"
exit 1

# while gh pr checks "$1" | grep -v tide | awk -v FS=$'\t'  '{print $2}' | grep -q pending ; do
#   printf "PR checks still pending, retrying in 10 seconds...\n"
#   sleep 30 # replace with 60000
# done

# if gh pr checks "$1" | awk -v FS=$'\t'  '{print $2}' | grep -q fail; then
#   printf "!!PR checks failed!!\n"
#   exit 1
# fi

# if gh pr checks "$1" | awk -v FS=$'\t'  '{print $2}' | grep -q pass; then
#   printf "!!PR checks passed!!\n"
#   exit 0
# fi