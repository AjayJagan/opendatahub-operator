#!/bin/bash

# This script calculates the previous version based on semantic versioning
# Usage: ./get-previous-version.sh <current_version>
# Example: ./get-previous-version.sh 2.30.0

if [ "$#" -ne 1 ]; then
    echo "Error: Version argument required"
    echo "Usage: $0 <version>"
    exit 1
fi

CURRENT_VERSION=$1

CURRENT_VERSION=${CURRENT_VERSION#v}

MAJOR=$(echo "$CURRENT_VERSION" | cut -d '.' -f 1)
MINOR=$(echo "$CURRENT_VERSION" | cut -d '.' -f 2)
PATCH=$(echo "$CURRENT_VERSION" | cut -d '.' -f 3)

if [ "$PATCH" -eq 0 ]; then
    if [ "$MINOR" -eq 0 ]; then
        PREV_MAJOR=$((MAJOR - 1))
        PREV_MINOR=9
        PREV_PATCH=0
    else
        PREV_PATCH=0
        PREV_MINOR=$((MINOR - 1))
        PREV_MAJOR=$MAJOR
    fi
else
    PREV_PATCH=$((PATCH - 1))
    PREV_MINOR=$MINOR
    PREV_MAJOR=$MAJOR
fi

echo "$PREV_MAJOR.$PREV_MINOR.$PREV_PATCH" 
