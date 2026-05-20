#!/bin/sh
# Unit tests for scripts/install.sh helper functions.
#
# Each test:
#   1. Sources install.sh in "library mode" (EZIDA_INSTALL_SOURCE_ONLY=1) so
#      that the helpers are defined but `main` is not invoked.
#   2. Stubs `uname`, `command -v`, `curl`, and the SHA256 tools as needed.
#   3. Asserts on the helper's output / exit status.
#
# Run with:  sh scripts/tests/test_install.sh

set -u

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
INSTALL_SH="${SCRIPT_DIR}/../install.sh"

PASS=0
FAIL=0

red()    { printf '\033[31m%s\033[0m\n' "$*"; }
green()  { printf '\033[32m%s\033[0m\n' "$*"; }

ok() {
    PASS=$((PASS + 1))
    green "  ok  $*"
}

bad() {
    FAIL=$((FAIL + 1))
    red   "  FAIL  $*"
}

# Run a test in a subshell so stubs do not leak across cases.
run_test() {
    name="$1"
    fn="$2"
    printf '%s\n' "${name}"
    (
        # Re-source in library mode for each test.
        EZIDA_INSTALL_SOURCE_ONLY=1
        export EZIDA_INSTALL_SOURCE_ONLY
        # shellcheck source=../install.sh
        . "${INSTALL_SH}"
        # The sourced script enables `set -eu`; tests need looser handling
        # so we can capture failures from helpers that call `die`.
        set +e
        "${fn}"
    )
    rc=$?
    if [ "${rc}" -eq 0 ]; then
        ok "${name}"
    else
        bad "${name}"
    fi
}

# --------------------------------------------------------------------------
# 2.2 — platform detection
# --------------------------------------------------------------------------

t_darwin_arm64() {
    uname() { printf 'Darwin arm64\n'; }
    out="$(detect_platform)"
    [ "${out}" = "darwin_arm64" ] || { printf '  got: %s\n' "${out}" >&2; return 1; }
}

t_linux_amd64() {
    uname() { printf 'Linux x86_64\n'; }
    out="$(detect_platform)"
    [ "${out}" = "linux_amd64" ] || { printf '  got: %s\n' "${out}" >&2; return 1; }
}

t_linux_aarch64() {
    uname() { printf 'Linux aarch64\n'; }
    out="$(detect_platform)"
    [ "${out}" = "linux_arm64" ] || { printf '  got: %s\n' "${out}" >&2; return 1; }
}

t_unsupported() {
    uname() { printf 'Linux i686\n'; }
    err="$(detect_platform 2>&1 1>/dev/null || true)"
    case "${err}" in
        *i686*) ;;
        *) printf '  stderr missing i686: %s\n' "${err}" >&2; return 1 ;;
    esac
    case "${err}" in
        *darwin/arm64*darwin/amd64*linux/arm64*linux/amd64*) ;;
        *) printf '  stderr missing platform list: %s\n' "${err}" >&2; return 1 ;;
    esac
}

# --------------------------------------------------------------------------
# 2.3 — version resolution
# --------------------------------------------------------------------------

t_version_override() {
    EZIDA_VERSION="v0.2.0"
    out="$(resolve_version)"
    [ "${out}" = "v0.2.0" ] || { printf '  got: %s\n' "${out}" >&2; return 1; }
}

t_version_from_api() {
    # Stub curl to return a fake API payload.
    curl() {
        printf '{\n  "tag_name": "v1.2.3",\n  "name": "v1.2.3"\n}\n'
    }
    unset EZIDA_VERSION 2>/dev/null || true
    out="$(resolve_version)"
    [ "${out}" = "v1.2.3" ] || { printf '  got: %s\n' "${out}" >&2; return 1; }
}

# --------------------------------------------------------------------------
# 2.4 — sha256 helper auto-detection
# --------------------------------------------------------------------------

t_sha256_missing() {
    # Use an empty PATH directory so neither sha256sum nor shasum resolves.
    EMPTY_DIR="$(mktemp -d)"
    ORIG_PATH="${PATH}"
    trap 'PATH="${ORIG_PATH}"; rm -rf "${EMPTY_DIR}"' EXIT
    PATH="${EMPTY_DIR}"
    err="$(setup_sha256 2>&1 1>/dev/null || true)"
    case "${err}" in
        *sha256sum*shasum*|*shasum*sha256sum*) ;;
        *) printf '  stderr missing tool names: %s\n' "${err}" >&2; return 1 ;;
    esac
}

# --------------------------------------------------------------------------
# 2.5 — checksum verification (mismatch aborts)
# --------------------------------------------------------------------------

t_checksum_mismatch() {
    setup_sha256
    TMPDIR="$(mktemp -d)"
    trap 'rm -rf "${TMPDIR}"' EXIT

    tarball="ezida_v0.0.0-test_linux_amd64.tar.gz"
    printf 'fake tarball contents\n' > "${TMPDIR}/${tarball}"

    # Pretend the published checksum is a known-wrong hex.
    bad_hash="0000000000000000000000000000000000000000000000000000000000000000"
    printf '%s  %s\n' "${bad_hash}" "${tarball}" > "${TMPDIR}/checksums.txt"

    # Stub curl: short-circuit by pre-populating files; replace with a no-op.
    curl() { return 0; }

    err="$(download_and_verify "v0.0.0-test" "linux_amd64" 2>&1 1>/dev/null || true)"
    case "${err}" in
        *"Checksum verification failed"*) ;;
        *) printf '  stderr missing message: %s\n' "${err}" >&2; return 1 ;;
    esac
}

t_checksum_match() {
    setup_sha256
    TMPDIR="$(mktemp -d)"
    trap 'rm -rf "${TMPDIR}"' EXIT

    tarball="ezida_v0.0.0-test_linux_amd64.tar.gz"
    printf 'fake tarball contents\n' > "${TMPDIR}/${tarball}"

    good_hash="$(compute_sha "${TMPDIR}/${tarball}")"
    printf '%s  %s\n' "${good_hash}" "${tarball}" > "${TMPDIR}/checksums.txt"

    curl() { return 0; }

    download_and_verify "v0.0.0-test" "linux_amd64" >/dev/null 2>&1 \
        || { printf '  expected zero exit\n' >&2; return 1; }
}

# --------------------------------------------------------------------------
# 2.7 — PATH reminder
# --------------------------------------------------------------------------

t_path_reminder_shown() {
    PATH="/usr/bin:/bin"
    HOME="/tmp/ezida-test-home"
    out="$(path_reminder)"
    case "${out}" in
        *"add \$HOME/.local/bin to your PATH"*) ;;
        *) printf '  expected reminder, got: %s\n' "${out}" >&2; return 1 ;;
    esac
}

t_path_reminder_hidden() {
    HOME="/tmp/ezida-test-home"
    PATH="/usr/bin:${HOME}/.local/bin:/bin"
    out="$(path_reminder)"
    [ -z "${out}" ] || { printf '  expected empty, got: %s\n' "${out}" >&2; return 1; }
}

# --------------------------------------------------------------------------
# Driver
# --------------------------------------------------------------------------

run_test "2.2 detect_platform darwin_arm64"    t_darwin_arm64
run_test "2.2 detect_platform linux_amd64"     t_linux_amd64
run_test "2.2 detect_platform linux_arm64"     t_linux_aarch64
run_test "2.2 detect_platform unsupported"     t_unsupported
run_test "2.3 resolve_version EZIDA_VERSION"   t_version_override
run_test "2.3 resolve_version from API"        t_version_from_api
run_test "2.4 setup_sha256 missing tools"      t_sha256_missing
run_test "2.5 checksum mismatch aborts"        t_checksum_mismatch
run_test "2.5 checksum match passes"           t_checksum_match
run_test "2.7 path reminder shown"             t_path_reminder_shown
run_test "2.7 path reminder hidden"            t_path_reminder_hidden

printf '\n'
printf 'passed: %d  failed: %d\n' "${PASS}" "${FAIL}"

[ "${FAIL}" -eq 0 ]
