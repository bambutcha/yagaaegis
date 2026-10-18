А#!/usr/bin/env bash
# Скачивает ClamAV runtime в outdir/runtime/<os>-<arch>/
# Использование: ./scripts/fetch-runtime.sh <linux|windows> <amd64> <outdir>
set -euo pipefail

OS="${1:?os: linux|windows}"
ARCH="${2:?arch: amd64}"
OUT="$(cd "${3:?outdir}" && pwd)"
PLATFORM="${OS}-${ARCH}"
DEST="${OUT}/runtime/${PLATFORM}"
mkdir -p "$DEST"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "==> runtime для ${PLATFORM} → ${DEST}"

if [[ "$OS" == "windows" ]]; then
  VER="${CLAMAV_VERSION:-1.4.2}"
  ZIP="${TMP}/clamav.zip"
  URLS=(
    "https://github.com/Cisco-Talos/clamav/releases/download/clamav-${VER}/clamav-${VER}.win.x64.zip"
    "https://www.clamav.net/downloads/production/clamav-${VER}.win.x64.zip"
  )
  ok=0
  for url in "${URLS[@]}"; do
    if curl -fsSL -o "$ZIP" "$url"; then
      ok=1
      break
    fi
  done
  if [[ "$ok" -ne 1 ]]; then
    echo "Не удалось скачать Windows ClamAV ${VER}"
    exit 1
  fi
  unzip -q "$ZIP" -d "${TMP}/win"
  ROOT="$(find "${TMP}/win" -maxdepth 3 -type f -name 'clamscan.exe' | head -1 | xargs -r dirname)"
  if [[ -z "$ROOT" ]]; then
    echo "clamscan.exe не найден в архиве"
    exit 1
  fi
  cp -a "${ROOT}/." "$DEST/"
  test -f "${DEST}/clamscan.exe"
  echo "OK windows → ${DEST}/clamscan.exe"
  exit 0
fi

if [[ "$OS" == "linux" && "$ARCH" == "amd64" ]]; then
  cd "$TMP"
  if ! command -v apt-get >/dev/null || ! command -v dpkg-deb >/dev/null; then
    echo "Нужны apt-get и dpkg-deb (Ubuntu/Debian)"
    exit 1
  fi
  # Без apt-get update / без sudo — качаем .deb в TMP
  apt-get download clamav clamav-base clamav-freshclam clamav-daemon clamdscan
  apt-get download libclamav12 2>/dev/null || apt-get download 'libclamav*' || true
  for dep in libtfm1 libjson-c5 libpcre2-8-0 libbz2-1.0 zlib1g libxml2 libcurl4t64 libcurl4 \
    libssl3t64 libssl3 libnghttp2-14 libidn2-0 librtmp1 libssh-4 libpsl5t64 libpsl5 \
    libgssapi-krb5-2 libldap2 liblber2 libbrotli1 libzstd1 libsasl2-2; do
    apt-get download "$dep" 2>/dev/null || true
  done
  if ! ls ./*.deb >/dev/null 2>&1; then
    echo "Не удалось скачать .deb ClamAV"
    exit 1
  fi
  mkdir -p extract
  for deb in ./*.deb; do
    dpkg-deb -x "$deb" extract/
  done
  CLAMSCAN="$(find extract -type f -name clamscan | head -1)"
  if [[ -z "$CLAMSCAN" ]]; then
    echo "clamscan не найден в .deb"
    exit 1
  fi
  cp -a "$CLAMSCAN" "$DEST/clamscan"
  # clamd + clamdscan — базы в памяти между сканами (ускорение)
  for bin in clamdscan clamd; do
    B="$(find extract -type f -name "$bin" | head -1)"
    if [[ -n "$B" ]]; then
      cp -a "$B" "$DEST/$bin"
      chmod +x "$DEST/$bin"
    fi
  done
  # Файлы и симлинки SONAME (libclamav.so.12 → libclamav.so.12.1.0)
  # -exec по одному: pipe+cp иногда segfault на битых symlink из .deb
  find extract \( -type f -o -type l \) -name '*.so*' -exec cp -a {} "$DEST/" \;

  # CVD CA (ClamAV 1.4+). Без этого после удаления системного clamav — «Broken CVD».
  mkdir -p "${DEST}/certs"
  if [[ -f extract/etc/clamav/certs/clamav.crt ]]; then
    cp -a extract/etc/clamav/certs/. "${DEST}/certs/"
  else
    echo "предупреждение: clamav.crt не найден в .deb"
  fi

  chmod +x "$DEST/clamscan"
  test -x "$DEST/clamscan"

  if ! CVD_CERTS_DIR="${DEST}/certs" LD_LIBRARY_PATH="$DEST" "$DEST/clamscan" --version >/dev/null; then
    echo "clamscan не стартует из runtime (проверь .so / symlink)"
    LD_LIBRARY_PATH="$DEST" ldd "$DEST/clamscan" || true
    exit 1
  fi
  echo "OK linux → ${DEST}/clamscan ($(find "$DEST" -name '*.so*' | wc -l) libs, certs=$(ls "${DEST}/certs" | wc -l), clamd=$([[ -x $DEST/clamd ]] && echo yes || echo no))"
  CVD_CERTS_DIR="${DEST}/certs" LD_LIBRARY_PATH="$DEST" "$DEST/clamscan" --version | head -1
  exit 0
fi

echo "Неподдерживаемая платформа: ${PLATFORM}"
exit 1
