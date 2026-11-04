#!/bin/bash
# Checks the Finder extension's menu logic against the menu.json that the
# app wrote (run the app once first), outside Finder.
set -euo pipefail
cd "$(dirname "$0")/.."
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
swiftc -parse-as-library -framework FinderSync extension/FinderSync.swift extension/MenuCheck.swift -o "$work/check"
touch "$work/a.txt" "$work/pic.png"
"$work/check" "$work"
