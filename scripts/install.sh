#!/usr/bin/env bash
#
# Standalone installer — downloads the latest overseer binary to ~/bin/.
# No repo clone required. Safe to run multiple times (idempotent).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/arthurvasconcelos/overseer/main/scripts/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/arthurvasconcelos/overseer/main/scripts/install.sh | bash -s -- --channel beta
#
# After installing, run:
#   overseer init           — configure brain_path and machine-local settings
#   overseer brain init     — scaffold your brain directory
#   overseer brain setup    — wire dotfiles and install Brewfile packages

set -euo pipefail

INSTALL_DIR="${HOME}/bin"
GITHUB_REPO="arthurvasconcelos/overseer"
BINARY_NAME="overseer"
CHANNEL="stable"

while [[ $# -gt 0 ]]; do
    case "${1}" in
        --channel)
            CHANNEL="${2}"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done

if [[ "${CHANNEL}" != "stable" && "${CHANNEL}" != "beta" ]]; then
    >&2 echo "  [error]  unknown channel: ${CHANNEL} (valid: stable, beta)"
    exit 1
fi

function detect_platform {
    local OS
    local ARCH
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    case "${ARCH}" in
        x86_64) ARCH="amd64" ;;
        aarch64 | arm64) ARCH="arm64" ;;
        *)
            >&2 echo "  [error]  Unsupported architecture: ${ARCH}"
            exit 1
            ;;
    esac

    echo "${OS}_${ARCH}"
}

function install_binary {
    local PLATFORM
    PLATFORM="$(detect_platform)"

    echo "  [binary] detecting platform: ${PLATFORM}"

    local LATEST_TAG
    if [[ "${CHANNEL}" == "beta" ]]; then
        LATEST_TAG="$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases" \
            | grep '"tag_name"' \
            | grep -E '-(beta|rc|alpha)' \
            | head -1 \
            | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
    else
        LATEST_TAG="$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
            | grep '"tag_name"' \
            | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
    fi

    if [[ "${LATEST_TAG}" == "" ]]; then
        >&2 echo "  [error]  could not fetch latest ${CHANNEL} release tag from GitHub"
        exit 1
    fi

    echo "  [binary] latest release: ${LATEST_TAG}"

    local CURRENT_VERSION=""
    if command -v "${BINARY_NAME}" &>/dev/null; then
        CURRENT_VERSION="$("${BINARY_NAME}" --version 2>/dev/null | awk '{print $2}')"
    elif [[ -x "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
        CURRENT_VERSION="$("${INSTALL_DIR}/${BINARY_NAME}" --version 2>/dev/null | awk '{print $2}')"
    fi

    if [[ "${CURRENT_VERSION}" == "${LATEST_TAG#v}" ]]; then
        echo "  [skip]   ${BINARY_NAME} ${CURRENT_VERSION} already up to date"
        return
    fi

    local ARCHIVE="${BINARY_NAME}_${LATEST_TAG#v}_${PLATFORM}.tar.gz"
    local DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${LATEST_TAG}/${ARCHIVE}"
    local TMP_DIR
    TMP_DIR="$(mktemp -d)"

    echo "  [binary] downloading ${ARCHIVE}..."
    if ! curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DIR}/${ARCHIVE}"; then
        >&2 echo "  [error]  download failed: ${DOWNLOAD_URL}"
        rm -rf "${TMP_DIR}"
        exit 1
    fi

    tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "${TMP_DIR}"
    mkdir -p "${INSTALL_DIR}"
    mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    rm -rf "${TMP_DIR}"

    echo "  [done]   ${INSTALL_DIR}/${BINARY_NAME} (${LATEST_TAG})"
}

echo "overseer install"
echo ""

install_binary

echo ""
echo "Make sure ${INSTALL_DIR} is on your PATH, then run:"
echo "  overseer init        — configure brain_path and machine-local settings"
echo "  overseer brain init  — scaffold your brain directory"
