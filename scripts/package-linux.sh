#!/usr/bin/env bash
set -euo pipefail

tag=${1:-}
if [[ ! "$tag" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ ]]; then
    printf 'Expected a version tag such as v0.1.0 or v0.1.0-rc.1\n' >&2
    exit 2
fi

version=${tag#v}
deb_version=${version/-/\~}
output_dir=${2:-dist}
mkdir -p -- "$output_dir"
output_dir=$(cd -- "$output_dir" && pwd)
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."

stage=$(mktemp -d)
trap 'rm -rf -- "$stage"' EXIT
assets=()

for arch in amd64 arm64; do
    name="eyeloveu-${tag}-linux-${arch}"
    archive="$stage/$name"
    package="$stage/deb-$arch"
    GOOS=linux GOARCH="$arch" CGO_ENABLED=0 make install \
        BIN_DIR="$archive" DESTDIR="$package" PREFIX=/usr DATA_DIR=/usr/share
    install -m644 README.md "$archive/README.md"
    tar -C "$stage" -czf "$output_dir/$name.tar.gz" "$name"

    install -d -m755 "$package/DEBIAN"
    cat > "$package/DEBIAN/control" <<EOF
Package: eyeloveu
Version: $deb_version
Architecture: $arch
Maintainer: EyeLoveU contributors <noreply@github.com>
Section: utils
Priority: optional
Depends: libgtk-4-1 (>= 4.10), libadwaita-1-0 (>= 1.3)
Description: Screen break and water reminders for Linux
 Native desktop reminders with a GTK interface and a lightweight Go daemon.
EOF
    deb="eyeloveu_${version}_${arch}.deb"
    dpkg-deb --root-owner-group --build "$package" "$output_dir/$deb"
    assets+=("$name.tar.gz" "$deb")
done

cd -- "$output_dir"
sha256sum -- "${assets[@]}" > SHA256SUMS
