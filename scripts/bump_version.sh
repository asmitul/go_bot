#!/usr/bin/env bash

set -euo pipefail

mode="${1:-patch}"
version_file="${2:-VERSION}"

if [[ ! -f "${version_file}" ]]; then
	echo "version file not found: ${version_file}" >&2
	exit 1
fi

current="$(tr -d '[:space:]' < "${version_file}")"
current="${current#v}"

if [[ ! "${current}" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
	echo "invalid semantic version in ${version_file}: ${current}" >&2
	exit 1
fi

major="${BASH_REMATCH[1]}"
minor="${BASH_REMATCH[2]}"
patch="${BASH_REMATCH[3]}"

case "${mode}" in
	major)
		major=$((major + 1))
		minor=0
		patch=0
		;;
	minor)
		minor=$((minor + 1))
		patch=0
		;;
	patch)
		patch=$((patch + 1))
		;;
	*)
		echo "usage: $0 [major|minor|patch] [version_file]" >&2
		exit 1
		;;
esac

new_version="${major}.${minor}.${patch}"
printf '%s\n' "${new_version}" > "${version_file}"

echo "${current} -> ${new_version}"
