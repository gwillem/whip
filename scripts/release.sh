#!/bin/bash

set -e

cd $(dirname "$0")/..

if [ -z "$1" ]; then
    # Get the latest tag
    git fetch --tags
    latest_tag=$(git describe --tags --abbrev=0)
    
    # Extract the version number and increment it
    current_version=${latest_tag#v}
    next_version=$(echo $current_version | awk -F. '{$NF = $NF + 1;} 1' | sed 's/ /./g')
    
    # Use the incremented version
    version="v$next_version"
    echo "No version provided. Using next version: $version"
else
    version=$1
fi

export version

./scripts/buildall.sh

gzip -9fk build/*/whip

gh release create $version --generate-notes build/github/*
# Pull new tags after creating release
git fetch --tags

cat <<'EOM'

base=https://github.com/gwillem/whip/releases/latest/download/whip
curl -L $base-$(uname -s)-$(uname -m).gz|gzip -d>whip&&chmod +x whip

EOM
