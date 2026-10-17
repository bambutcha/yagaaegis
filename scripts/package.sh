#!/usr/bin/env bash
# Самодостаточный дистрибутив: LocalGuard + runtime/ + seed/
#   ./scripts/package.sh linux amd64
#   ./scripts/package.sh windows amd64 [--nsis]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OS="${1:-linux}"
ARCH="${2:-amd64}"
WANT_NSIS=0
for a in "${@:3}"; do
  [[ "$a" == "--nsis" ]] && WANT_NSIS=1
done

PLATFORM="${OS}-${ARCH}"
DIST="${ROOT}/dist/${PLATFORM}"
BIN_NAME="LocalGuard"
[[ "$OS" == "windows" ]] && BIN_NAME="LocalGuard.exe"

cd "$ROOT"
export PATH="${PATH}:$(go env GOPATH)/bin"
rm -rf "$DIST"
mkdir -p "$DIST" "${ROOT}/build/bin"

echo "==> 1/5 wails build (${PLATFORM})"
if [[ "$OS" == "windows" ]]; then
  wails build -platform windows/amd64 -clean
else
  wails build -platform linux/amd64 -clean
fi

if [[ ! -f "${ROOT}/build/bin/${BIN_NAME}" ]]; then
  echo "нет ${ROOT}/build/bin/${BIN_NAME}"
  exit 1
fi
cp -a "${ROOT}/build/bin/${BIN_NAME}" "${DIST}/"

echo "==> 1b/5 Uninstall (linux+windows)"
UNINST_NAME="Uninstall"
[[ "$OS" == "windows" ]] && UNINST_NAME="Uninstall.exe"
(
  cd "$ROOT"
  GOOS="$OS" GOARCH="$ARCH" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
    -o "${DIST}/${UNINST_NAME}" ./cmd/uninstall
)
cp -a "${DIST}/${UNINST_NAME}" "${ROOT}/build/bin/${UNINST_NAME}"

echo "==> 2/5 ClamAV runtime"
bash "${ROOT}/scripts/fetch-runtime.sh" "$OS" "$ARCH" "$DIST"

echo "==> 3/5 seed CVD"
bash "${ROOT}/scripts/fetch-seed.sh" "$DIST"

echo "==> 4/5 stage в build/bin (для NSIS)"
rm -rf "${ROOT}/build/bin/runtime" "${ROOT}/build/bin/seed"
cp -a "${DIST}/runtime" "${ROOT}/build/bin/runtime"
cp -a "${DIST}/seed" "${ROOT}/build/bin/seed"

if [[ "$OS" == "windows" && "$WANT_NSIS" -eq 1 ]]; then
  echo "==> NSIS installer (runtime+seed уже в build/bin)"
  if command -v makensis >/dev/null; then
    wails build -platform windows/amd64 -nsis
  else
    echo "makensis не найден — только portable zip (на CI windows-latest будет installer)"
  fi
fi

echo "==> 5/5 архив"
mkdir -p "${ROOT}/dist"
if [[ "$OS" == "windows" ]]; then
  ARCHIVE="${ROOT}/dist/LocalGuard-${PLATFORM}.zip"
  (cd "${ROOT}/dist" && rm -f "$ARCHIVE" && zip -qr "$ARCHIVE" "${PLATFORM}")
  # копируем installer в dist/
  shopt -s nullglob
  for f in "${ROOT}/build/bin/"*-installer.exe; do
    cp -a "$f" "${ROOT}/dist/"
    echo "installer: $f"
  done
else
  ARCHIVE="${ROOT}/dist/LocalGuard-${PLATFORM}.tar.gz"
  (cd "${ROOT}/dist" && tar -czf "$ARCHIVE" "${PLATFORM}")
fi

echo ""
echo "Готово: ${ARCHIVE}"
ls -lh "${DIST}/${BIN_NAME}" "${DIST}/seed" 2>/dev/null | head
