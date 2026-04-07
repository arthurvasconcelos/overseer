#!/usr/bin/env bash
#
# Developer convenience script.
# Symlinks the brain repo into repos/brain so overseer can find it during
# local development without needing OVERSEER_BRAIN set.
#
# Safe to run multiple times (idempotent).

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BRAIN_DEFAULT="${HOME}/brain"
BRAIN_LINK="${REPO_ROOT}/repos/brain"

BRAIN_PATH="${OVERSEER_BRAIN:-${BRAIN_DEFAULT}}"

if [[ ! -d "${BRAIN_PATH}" ]]; then
    >&2 echo "error: brain directory not found at ${BRAIN_PATH}"
    >&2 echo "       set OVERSEER_BRAIN or create ${BRAIN_DEFAULT}"
    exit 1
fi

if [[ -L "${BRAIN_LINK}" ]]; then
    CURRENT="$(readlink "${BRAIN_LINK}")"
    if [[ "${CURRENT}" == "${BRAIN_PATH}" ]]; then
        echo "[skip] ${BRAIN_LINK} → ${BRAIN_PATH} already correct"
        exit 0
    else
        echo "[warn] ${BRAIN_LINK} points to ${CURRENT}, relinking..."
        rm "${BRAIN_LINK}"
    fi
elif [[ -e "${BRAIN_LINK}" ]]; then
    >&2 echo "error: ${BRAIN_LINK} exists and is not a symlink — remove it manually"
    exit 1
fi

mkdir -p "${REPO_ROOT}/repos"
ln -s "${BRAIN_PATH}" "${BRAIN_LINK}"
echo "[link] ${BRAIN_LINK} → ${BRAIN_PATH}"
