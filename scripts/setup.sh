#!/usr/bin/env bash
#
# Developer bootstrap — builds overseer from source and installs the binary.
# Requires a local clone of the repo and Go 1.21+.
#
# For end-user install (no repo clone needed), use install.sh instead:
#   curl -fsSL https://raw.githubusercontent.com/arthurvasconcelos/overseer/main/scripts/install.sh | bash
#
# After installing, run:
#   overseer init           — configure brain_path and machine-local settings
#   overseer brain init     — scaffold your brain directory
#   overseer brain setup    — wire dotfiles and install Brewfile packages

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_DIR="${HOME}/bin"
BINARY_NAME="overseer"

function build_binary {
    if ! command -v go &>/dev/null; then
        >&2 echo "  [error]  Go is not installed. Install Go 1.21+ to build from source."
        >&2 echo "           Or use install.sh to download a pre-built binary."
        exit 1
    fi

    echo "  [build]  building from source (${REPO_ROOT}/cli)..."
    mkdir -p "${INSTALL_DIR}"
    (cd "${REPO_ROOT}/cli" && go build -o "${INSTALL_DIR}/${BINARY_NAME}" .)
    echo "  [done]   ${INSTALL_DIR}/${BINARY_NAME} (built from source)"
}

echo "overseer dev setup"
echo "  REPO_ROOT: ${REPO_ROOT}"
echo ""

build_binary

echo ""
echo "Run 'overseer init' to configure your brain_path and machine-local settings."
