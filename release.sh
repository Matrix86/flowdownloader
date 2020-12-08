#!/bin/bash

usage()
{
  echo "Usage:"
  echo "./release.sh"
  exit 1
}

log()
{
  dt=$(date '+%d/%m/%Y %H:%M:%S');
  echo
  echo -e "\e[1m\e[32m[$dt] $1\e[0m"
}

error()
{
  dt=$(date '+%d/%m/%Y %H:%M:%S');
  echo -e "\e[1m\e[31m[$dt] $1\e[0m"
  usage
  exit 1
}

choose()
{
  echo
  echo -e -n "\e[1m\e[32m$1\e[0m"
}

CURRENT_VERSION=$(cat utils/version.go | grep "Version" | cut -d '"' -f 2)
TO_UPDATE=(
    utils/version.go
)

GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
GIT_COMMIT=$(git rev-parse --short HEAD)

choose "Current version is $CURRENT_VERSION, select new version: "
read NEW_VERSION
log "Creating version $NEW_VERSION ..."

for file in "${TO_UPDATE[@]}"
do
    log "Patching $file ..."
    sed -i.bak "s/$CURRENT_VERSION/$NEW_VERSION/g" "$file"
    rm -rf "$file.bak"
    git add $file
done

# Commit updated file with the new version
git commit -m "Releasing v$NEW_VERSION"
git push

# Create TAG on the repository
git tag -a v$NEW_VERSION -m "Release v$NEW_VERSION"
git push origin v$NEW_VERSION

log "Released on github"
