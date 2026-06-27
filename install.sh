#!/usr/bin/env sh
# install.sh — install hadoop-dev for Linux and macOS
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/yassine20011/hadoop-quickstart-docker/main/install.sh | sh
#
# Optional env overrides:
#   HADOOP_DEV_VERSION  — install a specific version (default: latest)
#   HADOOP_DEV_DIR      — install directory (default: ~/.local/bin)

set -e

REPO="yassine20011/hadoop-quickstart-docker"
BINARY="hadoop-dev"
INSTALL_DIR="${HADOOP_DEV_DIR:-$HOME/.local/bin}"

# ── Helpers ──────────────────────────────────────────────────────────────────

info()  { printf '  \033[2m%s\033[0m\n' "$*"; }
ok()    { printf '  \033[32m✓\033[0m %s\n' "$*"; }
warn()  { printf '  \033[33m⚠\033[0m %s\n' "$*" >&2; }
die()   { printf '  \033[31m✗\033[0m %s\n' "$*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || die "Required tool not found: $1"
}

download() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$1"
  else
    die "Neither curl nor wget found. Install one and retry."
  fi
}

download_file() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$2" "$1"
  else
    wget -qO "$2" "$1"
  fi
}

# ── Detect OS and arch ───────────────────────────────────────────────────────

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)      die "Unsupported OS: $OS — use install.ps1 on Windows" ;;
esac

case "$ARCH" in
  x86_64 | amd64) ARCH="amd64" ;;
  arm64 | aarch64) ARCH="arm64" ;;
  *) die "Unsupported architecture: $ARCH" ;;
esac

# ── Resolve version ───────────────────────────────────────────────────────────

if [ -n "$HADOOP_DEV_VERSION" ]; then
  VERSION="$HADOOP_DEV_VERSION"
else
  info "Fetching latest release..."
  API_URL="https://api.github.com/repos/${REPO}/releases/latest"
  VERSION=$(download "$API_URL" | grep '"tag_name"' | sed 's/.*"tag_name": *"v\([^"]*\)".*/\1/')
  [ -n "$VERSION" ] || die "Could not determine latest version. Check your internet connection."
fi

ok "Installing hadoop-dev v${VERSION} (${OS}/${ARCH})"

# ── Download and extract ──────────────────────────────────────────────────────

ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ARCHIVE}"
TMP_DIR=$(mktemp -d)
TMP_ARCHIVE="${TMP_DIR}/${ARCHIVE}"

info "Downloading ${ARCHIVE}..."
download_file "$URL" "$TMP_ARCHIVE" || die "Download failed: ${URL}"

info "Extracting..."
tar -xzf "$TMP_ARCHIVE" -C "$TMP_DIR"

# ── Install ───────────────────────────────────────────────────────────────────

mkdir -p "$INSTALL_DIR"
cp "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
chmod +x "${INSTALL_DIR}/${BINARY}"
rm -rf "$TMP_DIR"

ok "Installed to ${INSTALL_DIR}/${BINARY}"

# ── PATH check ────────────────────────────────────────────────────────────────

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*)
    ok "hadoop-dev is ready — run: hadoop-dev --help"
    ;;
  *)
    warn "${INSTALL_DIR} is not in your PATH."
    printf '\n  Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):\n'
    printf '    export PATH="%s:$PATH"\n\n' "$INSTALL_DIR"
    ;;
esac
