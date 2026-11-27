#!/bin/sh
# Copy the home-screen widget layout into the MyGo Android library.
# mygo build rewrites that project but leaves extra resource files in place.
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
dest="$root/.mygo/android/mygo/src/main/res"
mkdir -p "$dest"
cp -a "$root/internal/homewidget/res/." "$dest/"
