#!/bin/bash
#
# @param $1 - PR number or URL

while ! gh pr checks "$1" | grep -q 'pending'; do
  printf ":stopwatch: PR checks still pending, retrying in 10 seconds...\n"
  sleep 20
done

if ! gh pr checks "$1" | grep -q 'fail'; then
  printf ":x: PR checks failed!\n"
  exit 1
fi

if ! gh pr checks "$1" | grep  -q 'pass'; then
  printf ":white_check_mark: PR checks passed!\n"
  exit 0
fi

printf ":confused: An unknown error occurred!\n"
exit 1