#!/usr/bin/env bash
# Builds the desktop app's Debian package (lazytrackit-desktop) from an
# already built lazytrackit-gui binary:
#
#   packaging/linux/build-deb.sh <tag> <path to lazytrackit-gui> <out dir>
#
# The package's library dependencies (GTK 3, WebKitGTK 4.1, and the
# minimum glibc) are worked out from the binary by dpkg-shlibdeps, so
# `apt install ./lazytrackit-desktop_*.deb` pulls in whatever is missing.
# Needs dpkg-dev, and must run on the distro the binary was built on.
set -euo pipefail

tag=${1:?usage: build-deb.sh <tag> <binary> <out dir>}
bin=${2:?usage: build-deb.sh <tag> <binary> <out dir>}
out=${3:?usage: build-deb.sh <tag> <binary> <out dir>}
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)

# Must match update.DebVersion: no "v", and "-rc1" → "~rc1" so a
# pre-release sorts before the release.
version=${tag#v}
version=${version//-/\~}
arch=$(dpkg --print-architecture)
pkg=lazytrackit-desktop

root=$(mktemp -d)
chmod 755 "$root"
trap 'rm -rf "$root"' EXIT
install -Dm755 "$bin" "$root/usr/bin/lazytrackit-gui"
install -Dm644 "$here/lazytrackit.desktop" "$root/usr/share/applications/lazytrackit.desktop"
install -Dm644 "$repo/internal/gui/icons/app.png" "$root/usr/share/icons/hicolor/256x256/apps/lazytrackit.png"

# dpkg-shlibdeps wants to run inside a source package; a bare
# debian/control is enough.
shl=$(mktemp -d)
mkdir "$shl/debian"
touch "$shl/debian/control"
depends=$(cd "$shl" && dpkg-shlibdeps -O "$root/usr/bin/lazytrackit-gui" 2>/dev/null | sed -n 's/^shlibs:Depends=//p')
rm -rf "$shl"
if [ -z "$depends" ]; then
  echo "dpkg-shlibdeps found no dependencies" >&2
  exit 1
fi

mkdir -p "$root/DEBIAN"
cat > "$root/DEBIAN/control" <<CONTROL
Package: $pkg
Version: $version
Architecture: $arch
Maintainer: Ahmed HABBACHI <ahmed-habbachi@users.noreply.github.com>
Installed-Size: $(du -sk --exclude=DEBIAN "$root" | cut -f1)
Depends: $depends
Section: utils
Priority: optional
Homepage: https://github.com/ahmed-habbachi/LazyTrackIt
Description: Time tracking desktop app (tray icon + window)
 LazyTrackIt logs your work time and checks you in and out against
 Case.TrackIt, Toggl Track or a week-booking (Zeiterfassung) server.
 This package installs the desktop app, lazytrackit-gui. When a new
 version is released, the app offers the new package for download.
CONTROL

mkdir -p "$out"
deb="$out/${pkg}_${version}_${arch}.deb"
dpkg-deb --build --root-owner-group "$root" "$deb" >/dev/null
echo "$deb"
