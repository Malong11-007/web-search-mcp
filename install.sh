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

# Detect OS and architecture
detect_platform() {
    local os arch

    case "$(uname -s)" in
        Darwin)  os="darwin" ;;
        Linux)   os="linux" ;;
        *)       err "Unsupported OS: $(uname -s)"; exit 1 ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)  arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *)             err "Unsupported architecture: $(uname -m)"; exit 1 ;;
    esac

    echo "${os}_${arch}"
}

# Check if Go is available and use go install as the easiest path
install_via_go() {
    if command -v go &>/dev/null; then
        info "Go detected — installing via go install..."
        go install "github.com/${REPO}/cmd/server@${VERSION}"
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

# Download pre-compiled binary from GitHub Releases
install_via_release() {
    local platform="$1"
    local archive="${BINARY}_${platform}.tar.gz"
    local url

    if [ "$VERSION" = "latest" ]; then
        url="https://github.com/${REPO}/releases/latest/download/${archive}"
    else
        url="https://github.com/${REPO}/releases/download/${VERSION}/${archive}"
    fi

    info "Downloading ${url}..."
    local tmpdir
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    if ! curl -fsSL "$url" -o "${tmpdir}/${archive}"; then
        err "Download failed. The release may not exist yet."
        err "Try building from source: git clone https://github.com/${REPO}.git && cd web-search-mcp && go build -o ${BINARY} ./cmd/server/"
        return 1
    fi

    tar -xzf "${tmpdir}/${archive}" -C "$tmpdir"

    local install_dir="${HOME}/.local/bin"
    mkdir -p "$install_dir"

    mv "${tmpdir}/${BINARY}" "${install_dir}/${BINARY}"
    chmod +x "${install_dir}/${BINARY}"

    ok "Installed to ${install_dir}/${BINARY}"

    if ! echo "$PATH" | grep -q "${install_dir}"; then
        info "Add ${install_dir} to your PATH:"
        info "  echo 'export PATH=\"${install_dir}:\$PATH\"' >> ~/.bashrc"
        info "  echo 'export PATH=\"${install_dir}:\$PATH\"' >> ~/.zshrc"
    fi
}

# Build from source (fallback)
install_from_source() {
    info "Building from source..."
    local tmpdir
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    git clone "https://github.com/${REPO}.git" "$tmpdir"
    cd "$tmpdir"
    go build -o "$BINARY" ./cmd/server/

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

    # Try downloading pre-built binary
    local platform
    platform="$(detect_platform)"
    if install_via_release "$platform"; then
        echo ""
        ok "Done! Configure Claude Code by adding to ~/.claude/settings.json:"
        echo ""
        echo '  {'
        echo '    "mcpServers": {'
        echo '      "web-search": {'
        echo '        "command": "'"${HOME}/.local/bin/${BINARY}"'",'
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
