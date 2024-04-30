#!/bin/bash
#
# @param $1 - PR number or URL
# wait for a bit until pr is created, otherwise it throws an error "no checks reported on the 'odh-release/e2e-test' branch"
set -euo pipefail

sleep 10

while $(gh pr checks "$1" | grep -v 'tide' | grep 'pending'); do
  printf "PR checks still pending, retrying in 10 seconds...\n"
  sleep 10 # To be changed to 600000(10 minutes)
done

if $(gh pr checks "$1" | grep 'fail'); then
  printf "!!PR checks failed!!\n"
  exit 1
fi

if $(gh pr checks "$1" | grep 'pass'); then
  printf "!!PR checks passed!!\n"
  exit 0
fi

printf "!!An unknown error occurred!!\n"
exit 1