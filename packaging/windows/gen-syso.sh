#!/usr/bin/env bash
# Builds cmd/lazytrackit-gui/rsrc_windows_amd64.syso, which the Go
# linker picks up for Windows builds: the app icon, manifest and version
# details for lazytrackit-gui.exe. Needs llvm-windres (or GNU windres).
#
#   packaging/windows/gen-syso.sh [version]    # e.g. v1.2.3; default 0.0.0
#
# The committed .syso is built with the default version; the release
# workflow rebuilds it with the tag's.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
out="$here/../../cmd/lazytrackit-gui/rsrc_windows_amd64.syso"

version=${1:-0.0.0}
num=${version#v}
num=${num%%[-+]*} # drop a pre-release/build suffix
IFS=. read -r major minor patch _ <<<"$num.0.0.0"
for n in "$major" "$minor" "$patch"; do
  [[ $n =~ ^[0-9]+$ ]] || { echo "gen-syso: can't read a version from $version" >&2; exit 1; }
done

windres=$(command -v llvm-windres || command -v x86_64-w64-mingw32-windres || command -v windres) ||
  { echo "gen-syso: needs llvm-windres or windres" >&2; exit 1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
sed -e "s/@NUMVER@/$major,$minor,$patch,0/g" -e "s/@VERSION@/$version/g" "$here/app.rc.in" >"$work/app.rc"
cp "$here/lazytrackit.ico" "$here/app.manifest" "$work/"
(cd "$work" && "$windres" --target=pe-x86-64 -O coff -i app.rc -o "$out")
echo "wrote $out ($version)"
