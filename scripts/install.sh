#!/bin/sh
# ezida-kanban install script
#
# Downloads the appropriate release tarball for the host platform from the
# GitHub Releases of nicolasvergoz/ezida-kanban, verifies its SHA256 against
# the published checksums.txt, and installs the `ezida` binary into
# ~/.local/bin/ezida (mode 0755). Idempotent: re-running upgrades in place.
#
# Usage:
#   curl -sSL https://github.com/nicolasvergoz/ezida-kanban/releases/latest/download/install.sh | sh
#
# Env overrides:
#   EZIDA_VERSION   pin a specific tag (e.g. v0.1.0); default: latest release.

set -eu

REPO="nicolasvergoz/ezida-kanban"
INSTALL_DIR="${HOME}/.local/bin"
TMPDIR=""

usage() {
    cat <<'USAGE'
Usage: install.sh

  Installs the ezida CLI into ~/.local/bin/ezida.

Environment:
  EZIDA_VERSION   pin a specific release tag (e.g. v0.1.0)
USAGE
}

die() {
    printf 'error: %s\n' "$*" >&2
    exit 1
}

cleanup() {
    if [ -n "${TMPDIR}" ] && [ -d "${TMPDIR}" ]; then
        rm -rf "${TMPDIR}"
    fi
}
trap cleanup EXIT INT TERM

# ---------------------------------------------------------------------------
# Argument parsing (only -h/--help for now).
# ---------------------------------------------------------------------------

parse_args() {
    while [ "$#" -gt 0 ]; do
        case "$1" in
            -h|--help)
                usage
                exit 0
                ;;
            *)
                usage >&2
                die "unknown argument: $1"
                ;;
        esac
    done
}

# ---------------------------------------------------------------------------
# Platform detection.
# ---------------------------------------------------------------------------

detect_platform() {
    UNAME="$(uname -sm)"
    case "${UNAME}" in
        "Darwin arm64")    OS_ARCH="darwin_arm64" ;;
        "Darwin x86_64")   OS_ARCH="darwin_amd64" ;;
        "Linux aarch64")   OS_ARCH="linux_arm64" ;;
        "Linux arm64")     OS_ARCH="linux_arm64" ;;
        "Linux x86_64")    OS_ARCH="linux_amd64" ;;
        *)
            die "unsupported platform: ${UNAME} — supported: darwin/arm64, darwin/amd64, linux/arm64, linux/amd64"
            ;;
    esac
    printf '%s' "${OS_ARCH}"
}

# ---------------------------------------------------------------------------
# SHA256 helper auto-detection.
# ---------------------------------------------------------------------------

setup_sha256() {
    if command -v sha256sum >/dev/null 2>&1; then
        SHA256_TOOL="sha256sum"
    elif command -v shasum >/dev/null 2>&1; then
        SHA256_TOOL="shasum"
    else
        die "need sha256sum or shasum -a 256 on PATH to verify the download"
    fi
}

compute_sha() {
    # $1: path to file. Echoes the lowercase hex digest only.
    if [ "${SHA256_TOOL}" = "sha256sum" ]; then
        sha256sum "$1" | awk '{print $1}'
    else
        shasum -a 256 "$1" | awk '{print $1}'
    fi
}

# ---------------------------------------------------------------------------
# Version resolution: $EZIDA_VERSION or latest tag from the GitHub API.
# ---------------------------------------------------------------------------

resolve_version() {
    if [ -n "${EZIDA_VERSION:-}" ]; then
        printf '%s' "${EZIDA_VERSION}"
        return 0
    fi

    command -v curl >/dev/null 2>&1 || die "curl is required"

    api_url="https://api.github.com/repos/${REPO}/releases/latest"
    tag="$(
        curl -fsSL "${api_url}" \
            | grep '"tag_name":' \
            | head -n 1 \
            | sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/'
    )" || die "failed to query latest release from ${api_url}"

    if [ -z "${tag}" ]; then
        die "could not parse tag_name from ${api_url}"
    fi
    printf '%s' "${tag}"
}

# ---------------------------------------------------------------------------
# Download + checksum verification.
# ---------------------------------------------------------------------------

download_and_verify() {
    # $1: version, $2: os_arch
    version="$1"
    os_arch="$2"

    tarball="ezida_${version}_${os_arch}.tar.gz"
    base_url="https://github.com/${REPO}/releases/download/${version}"
    tarball_url="${base_url}/${tarball}"
    checksums_url="${base_url}/checksums.txt"

    printf 'downloading %s\n' "${tarball_url}"
    curl -fsSL -o "${TMPDIR}/${tarball}" "${tarball_url}" \
        || die "failed to download ${tarball_url}"

    printf 'downloading %s\n' "${checksums_url}"
    curl -fsSL -o "${TMPDIR}/checksums.txt" "${checksums_url}" \
        || die "failed to download ${checksums_url}"

    actual="$(compute_sha "${TMPDIR}/${tarball}")"
    expected="$(grep "  ${tarball}\$" "${TMPDIR}/checksums.txt" | awk '{print $1}')"

    if [ -z "${expected}" ]; then
        die "Checksum verification failed: ${tarball} not listed in checksums.txt"
    fi
    if [ "${actual}" != "${expected}" ]; then
        printf 'expected: %s\n' "${expected}" >&2
        printf 'actual:   %s\n' "${actual}" >&2
        die "Checksum verification failed for ${tarball}"
    fi
}

# ---------------------------------------------------------------------------
# Install: extract tarball, copy binary to ~/.local/bin/ezida.
# ---------------------------------------------------------------------------

install_binary() {
    # $1: version, $2: os_arch
    version="$1"
    os_arch="$2"
    tarball="ezida_${version}_${os_arch}.tar.gz"

    extract_dir="${TMPDIR}/extract"
    mkdir -p "${extract_dir}"
    tar -xzf "${TMPDIR}/${tarball}" -C "${extract_dir}" \
        || die "failed to extract ${tarball}"

    if [ ! -f "${extract_dir}/ezida" ]; then
        die "tarball is missing the ezida binary"
    fi

    mkdir -p "${INSTALL_DIR}"
    cp "${extract_dir}/ezida" "${INSTALL_DIR}/ezida"
    chmod 0755 "${INSTALL_DIR}/ezida"

    # macOS AMFI rejects cross-compiled ad-hoc signatures (error -423).
    # Re-sign locally so the kernel accepts the binary.
    if [ "$(uname -s)" = "Darwin" ] && command -v codesign >/dev/null 2>&1; then
        codesign --force --sign - "${INSTALL_DIR}/ezida" >/dev/null 2>&1 \
            || printf 'warning: codesign re-sign failed; run: codesign --force --sign - %s/ezida\n' "${INSTALL_DIR}" >&2
    fi
}

# ---------------------------------------------------------------------------
# PATH reminder.
# ---------------------------------------------------------------------------

path_reminder() {
    case ":${PATH}:" in
        *":${HOME}/.local/bin:"*) ;;
        *)
            # shellcheck disable=SC2016
            printf 'note: add $HOME/.local/bin to your PATH\n'
            ;;
    esac
}

# ---------------------------------------------------------------------------
# Main.
# ---------------------------------------------------------------------------

main() {
    parse_args "$@"

    setup_sha256

    os_arch="$(detect_platform)"
    version="$(resolve_version)"

    TMPDIR="$(mktemp -d 2>/dev/null || mktemp -d -t ezida-install)"

    download_and_verify "${version}" "${os_arch}"
    install_binary "${version}" "${os_arch}"

    printf 'installed ezida %s to %s/ezida\n' "${version}" "${INSTALL_DIR}"
    path_reminder
}

if [ -z "${EZIDA_INSTALL_SOURCE_ONLY:-}" ]; then
    main "$@"
fi
