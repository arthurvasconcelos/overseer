#!/usr/bin/env bash
#
# Bootstrap script — installs the overseer binary and wires dotfiles via symlinks.
# Safe to run multiple times (idempotent).
#
# For Claude config and personal files, run: brain/scripts/setup.sh
#
# Binary install strategy:
#   1. Download pre-built binary from GitHub Releases (no Go required)
#   2. Fall back to go build if Go is available and download fails

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKUP_DIR="${HOME}/.overseer-backups/$(date '+%Y%m%d_%H%M%S')"
INSTALL_DIR="${HOME}/bin"
GITHUB_REPO="arthurvasconcelos/overseer"
BINARY_NAME="overseer"

function make_symlink {
    local SOURCE="${1}"
    local TARGET="${2}"

    if [[ -L "${TARGET}" ]]; then
        local CURRENT_DEST
        CURRENT_DEST="$(readlink "${TARGET}")"
        if [[ "${CURRENT_DEST}" == "${SOURCE}" ]]; then
            echo "  [skip]   ${TARGET} already correct"
            return
        else
            >&2 echo "  [warn]   ${TARGET} → symlink points elsewhere (${CURRENT_DEST}), skipping"
            return
        fi
    fi

    if [[ -e "${TARGET}" ]]; then
        mkdir -p "${BACKUP_DIR}"
        local BASENAME
        BASENAME="$(basename "${TARGET}")"
        mv "${TARGET}" "${BACKUP_DIR}/${BASENAME}"
        echo "  [backup] ${TARGET} → ${BACKUP_DIR}/${BASENAME}"
    fi

    local PARENT
    PARENT="$(dirname "${TARGET}")"
    mkdir -p "${PARENT}"
    ln -s "${SOURCE}" "${TARGET}"
    echo "  [link]   ${TARGET} → ${SOURCE}"
}

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
    LATEST_TAG="$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
        | grep '"tag_name"' \
        | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"

    if [[ "${LATEST_TAG}" == "" ]]; then
        >&2 echo "  [warn]   could not fetch latest release tag from GitHub"
        fallback_build
        return
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
        >&2 echo "  [warn]   download failed: ${DOWNLOAD_URL}"
        rm -rf "${TMP_DIR}"
        fallback_build
        return
    fi

    tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "${TMP_DIR}"
    mkdir -p "${INSTALL_DIR}"
    mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    rm -rf "${TMP_DIR}"

    echo "  [link]   ${INSTALL_DIR}/${BINARY_NAME} (${LATEST_TAG})"
}

function fallback_build {
    if ! command -v go &>/dev/null; then
        >&2 echo "  [error]  Go is not installed and binary download failed. Install Go or push a release tag."
        exit 1
    fi

    echo "  [build]  falling back to go build..."
    (cd "${REPO_ROOT}/cli" && go build -o "${INSTALL_DIR}/${BINARY_NAME}" .)
    echo "  [link]   ${INSTALL_DIR}/${BINARY_NAME} (built from source)"
}

echo "overseer setup"
echo "  REPO_ROOT: ${REPO_ROOT}"
echo ""

install_binary
echo ""

make_symlink "${REPO_ROOT}/dotfiles/shell/.zshrc"    "${HOME}/.zshrc"
make_symlink "${REPO_ROOT}/dotfiles/git/.gitconfig"  "${HOME}/.gitconfig"

echo ""
echo "Done."
