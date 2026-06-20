#!/usr/bin/env bash
set -euo pipefail

# web-search-mcp installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Malong11-007/web-search-mcp/main/install.sh | bash

REPO="Malong11-007/web-search-mcp"
BINARY="web-search-mcp"
VERSION="${VERSION:-latest}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

info()  { echo -e "${CYAN}→${NC} $*"; }
ok()    { echo -e "${GREEN}✓${NC} $*"; }
err()   { echo -e "${RED}✗${NC} $*" >&2; }

# Check if Go is available and use go install as the easiest path
install_via_go() {
    if command -v go &>/dev/null; then
        info "Go detected — installing via go install..."
        go install "github.com/${REPO}/cmd/web-search-mcp@${VERSION}"
        local bin_dir
        bin_dir="$(go env GOPATH)/bin"
        if [ -x "${bin_dir}/${BINARY}" ]; then
            ok "Installed to ${bin_dir}/${BINARY}"
            info "Make sure ${bin_dir} is in your PATH"
            return 0
        fi
    fi
    return 1
}

# Build from source (fallback)
install_from_source() {
    info "Building from source..."
    local tmpdir
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    git clone "https://github.com/${REPO}.git" "$tmpdir"
    cd "$tmpdir"
    go build -o "$BINARY" ./cmd/web-search-mcp/

    local install_dir="${HOME}/.local/bin"
    mkdir -p "$install_dir"
    mv "$BINARY" "${install_dir}/${BINARY}"
    chmod +x "${install_dir}/${BINARY}"

    ok "Built and installed to ${install_dir}/${BINARY}"
}

main() {
    echo ""
    info "Installing web-search-mcp..."
    echo ""

    # Try go install first (easiest)
    if install_via_go; then
        echo ""
        ok "Done! Configure Claude Code by adding to ~/.claude/settings.json:"
        echo ""
        echo '  {'
        echo '    "mcpServers": {'
        echo '      "web-search": {'
        echo '        "command": "web-search-mcp",'
        echo '        "args": []'
        echo '      }'
        echo '    }'
        echo '  }'
        exit 0
    fi

    # Fall back to building from source
    install_from_source
    echo ""
    ok "Done!"
}

main
