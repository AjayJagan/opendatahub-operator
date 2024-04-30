#!/bin/bash
#
# @param $1 - PR number or URL
# wait for a bit until pr is created, otherwise it throws an error "no checks reported on the 'odh-release/e2e-test' branch"
set -euo

sleep 10

pr_has_status() {
    local pr=$1
    local status=$2
    local skip=${3:-tide}

    gh pr checks $pr | awk -v FS=$'\t' -v status=$status "\$1 ~ /$skip/{next} \$2 == status {found=1} END {if (!found) exit 1}"
}

while pr_has_status $1 pending; do
  echo "PR checks still pending, retrying in 10 seconds..."
  sleep 30 # replace with 60000
done

pr_has_status $1 fail && { echo "!!PR checks failed!!"; exit 1; }
pr_has_status $1 pass && { echo "!!PR checks passed!!"; exit 0; }
echo "!!An unknown error occurred!!"
exit 1
