#!/usr/bin/env bash
# Качает CVD для офлайн первого запуска в seed/ (зеркало Microsoft — работает из РФ).
# Использование: ./scripts/fetch-seed.sh <outdir>
set -euo pipefail

OUT="${1:?outdir}"
SEED="${OUT}/seed"
MIRROR="${CLAMAV_MIRROR:-https://packages.microsoft.com/clamav}"
mkdir -p "$SEED"

echo "==> seed CVD из ${MIRROR} → ${SEED}"
for f in main.cvd daily.cvd bytecode.cvd; do
  echo "  $f"
  curl -fsSL -o "${SEED}/${f}.tmp" "${MIRROR}/${f}"
  mv "${SEED}/${f}.tmp" "${SEED}/${f}"
done
ls -lh "$SEED"
echo "OK seed"
