#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC_LINK="/tmp/go-dispatch-proxy-gui-appimage-src"

# CGO splits flags on spaces, so build through a temporary path without spaces.
ln -sfn "$ROOT_DIR" "$SRC_LINK"

BUILD_DIR="$SRC_LINK/build"
DEB_DIR="$BUILD_DIR/debs"
SYSROOT="$BUILD_DIR/sysroot"
APPDIR="$BUILD_DIR/appdir"
APPIMAGE_TOOL="$BUILD_DIR/appimagetool-x86_64.AppImage"
OUTPUT="$BUILD_DIR/Go_Dispatch_Proxy_GUI-x86_64.AppImage"

PACKAGES=(
	libxrandr-dev libxrandr2
	libxcursor-dev libxcursor1
	libxinerama-dev libxinerama1
	libxi-dev libxi6
	libxxf86vm-dev libxxf86vm1
	libxext-dev libxext6
	libxrender-dev libxrender1
	libxfixes-dev libxfixes3
	x11proto-dev
	libx11-dev libx11-6
)

mkdir -p "$DEB_DIR" "$SYSROOT"

(
	cd "$DEB_DIR"
	apt-get download "${PACKAGES[@]}"
)

for deb in "$DEB_DIR"/*.deb; do
	dpkg-deb -x "$deb" "$SYSROOT"
done

rm -rf "$APPDIR"
mkdir -p \
	"$APPDIR/usr/bin" \
	"$APPDIR/usr/lib/x86_64-linux-gnu" \
	"$APPDIR/usr/share/applications" \
	"$APPDIR/usr/share/icons/hicolor/256x256/apps"

(
	cd "$SRC_LINK"
	PKG_CONFIG_PATH="$SYSROOT/usr/lib/x86_64-linux-gnu/pkgconfig:$SYSROOT/usr/share/pkgconfig" \
	CGO_CFLAGS="-I$SYSROOT/usr/include" \
	CGO_LDFLAGS="-L$SYSROOT/usr/lib/x86_64-linux-gnu" \
	go build -ldflags="-s -w" -o "$APPDIR/usr/bin/dispatch-proxy-gui" .
)

cat >"$APPDIR/AppRun" <<'APPRUN'
#!/bin/sh
APPDIR="$(dirname "$(readlink -f "$0")")"
export LD_LIBRARY_PATH="$APPDIR/usr/lib:$APPDIR/usr/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
case "${LC_ALL:-}" in C|POSIX|C.UTF-8|C.utf8) unset LC_ALL ;; esac
case "${LC_CTYPE:-}" in C|POSIX|C.UTF-8|C.utf8) unset LC_CTYPE ;; esac
case "${LANG:-}" in ""|C|POSIX|C.UTF-8|C.utf8) export LANG=en_US.UTF-8 ;; esac
exec "$APPDIR/usr/bin/dispatch-proxy-gui" "$@"
APPRUN
chmod +x "$APPDIR/AppRun"

cat >"$APPDIR/dispatch-proxy-gui.desktop" <<'DESKTOP'
[Desktop Entry]
Type=Application
Name=Go Dispatch Proxy GUI
Comment=HTTP and SOCKS dispatch proxy GUI
Exec=dispatch-proxy-gui
Icon=dispatch-proxy-gui
Categories=Network;
Terminal=false
DESKTOP

cp "$APPDIR/dispatch-proxy-gui.desktop" "$APPDIR/usr/share/applications/dispatch-proxy-gui.desktop"
cp "$SRC_LINK/icon.png" "$APPDIR/dispatch-proxy-gui.png"
cp "$SRC_LINK/icon.png" "$APPDIR/usr/share/icons/hicolor/256x256/apps/dispatch-proxy-gui.png"

shopt -s nullglob
for pattern in \
	'libXrandr.so*' 'libXcursor.so*' 'libXinerama.so*' 'libXi.so*' 'libXxf86vm.so*' \
	'libXfixes.so*' 'libXrender.so*' 'libXext.so*' 'libX11.so*'
do
	for lib in "$SYSROOT/usr/lib/x86_64-linux-gnu"/$pattern; do
		cp -P "$lib" "$APPDIR/usr/lib/x86_64-linux-gnu/"
	done
done
shopt -u nullglob

if [ ! -x "$APPIMAGE_TOOL" ]; then
	curl -L --fail \
		-o "$APPIMAGE_TOOL" \
		"https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage"
	chmod +x "$APPIMAGE_TOOL"
fi

ARCH=x86_64 APPIMAGE_EXTRACT_AND_RUN=1 "$APPIMAGE_TOOL" "$APPDIR" "$OUTPUT"

printf 'Built: %s\n' "$ROOT_DIR/build/Go_Dispatch_Proxy_GUI-x86_64.AppImage"
