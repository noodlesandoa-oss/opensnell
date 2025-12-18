#!/usr/bin/env bash
set -euo pipefail

REPO="noodlesandoa-oss/opensnell"
REF="${OPEN_SNELL_REF:-main}"

# By default, installer will try to download a prebuilt binary from GitHub Releases:
#   https://github.com/${REPO}/releases/latest
# If that fails (no release / no matching asset), it will fall back to building from source.
#
# You can override behavior:
#   OPEN_SNELL_RELEASE=0   # skip release install, build from source
#   OPEN_SNELL_TAG=vX.Y.Z  # download assets from a specific release tag
#   OPEN_SNELL_ASSET=name  # force a specific asset name
OPEN_SNELL_RELEASE="${OPEN_SNELL_RELEASE:-1}"
OPEN_SNELL_TAG="${OPEN_SNELL_TAG:-}"
OPEN_SNELL_ASSET="${OPEN_SNELL_ASSET:-}"

# Optional: stage install under a directory (useful for testing/packaging).
# Example:
#   OPEN_SNELL_DESTDIR=/tmp/opensnell-root bash install.sh
OPEN_SNELL_DESTDIR="${OPEN_SNELL_DESTDIR:-}"

# Optional: skip calling systemctl (useful in containers or when DESTDIR is used).
OPEN_SNELL_SKIP_SYSTEMD="${OPEN_SNELL_SKIP_SYSTEMD:-}"

TARGET_BIN_PATH="/usr/local/bin/snell-server"
TARGET_CONFIG_DIR="/etc/open-snell"
TARGET_CONFIG_PATH="${TARGET_CONFIG_DIR}/snell-server.conf"
TARGET_UNIT_PATH="/etc/systemd/system/snell-server.service"

dest_path() {
  local p="$1"
  if [[ -n "$OPEN_SNELL_DESTDIR" ]]; then
    echo "${OPEN_SNELL_DESTDIR%/}${p}"
  else
    echo "$p"
  fi
}

BIN_INSTALL_PATH="$(dest_path "$TARGET_BIN_PATH")"
CONFIG_DIR="$(dest_path "$TARGET_CONFIG_DIR")"
CONFIG_PATH="$(dest_path "$TARGET_CONFIG_PATH")"
UNIT_PATH="$(dest_path "$TARGET_UNIT_PATH")"

log() {
  printf "%s\n" "$*" >&2
}

debug() {
  [[ "${OPEN_SNELL_DEBUG:-0}" == "1" ]] || return 0
  log "[debug] $*"
}

die() {
  log "ERROR: $*"
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

download() {
  local url="$1"
  local out="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$out" "$url"
    return
  fi
  die "need curl or wget to download $url"
}

try_download() {
  local url="$1"
  local out="$2"

  # Expose the last attempted URL / error for diagnostics.
  LAST_TRY_URL="$url"
  LAST_TRY_ERROR=""

  if command -v curl >/dev/null 2>&1; then
    local err
    err="$(curl -fsSL "$url" -o "$out" 2>&1)" || {
      LAST_TRY_ERROR="$err"
      debug "download failed (curl): url=$url err=$err"
      return 1
    }
    return 0
  fi
  if command -v wget >/dev/null 2>&1; then
    local err
    err="$(wget -qO "$out" "$url" 2>&1)" || {
      LAST_TRY_ERROR="$err"
      debug "download failed (wget): url=$url err=$err"
      return 1
    }
    return 0
  fi
  return 1
}

fetch_latest_tag() {
  # Returns the latest GitHub release tag (e.g. v0.1.1).
  # Prefer the redirect-based method (works even when GitHub API is rate-limited).
  local latest_url="https://github.com/${REPO}/releases/latest"
  local effective=""
  if command -v curl >/dev/null 2>&1; then
    local out rc
    out="$(curl -fsSL -o /dev/null -w '%{http_code} %{url_effective}' "$latest_url" 2>&1)" || rc=$?
    rc=${rc:-0}
    debug "latest tag probe (curl): rc=${rc} out='${out}'"
    if [[ "$rc" -eq 0 ]]; then
      effective="$(printf '%s' "$out" | awk '{print $2}')"
    fi
  elif command -v wget >/dev/null 2>&1; then
    # wget doesn't have url_effective; parse Location from headers.
    local out
    out="$(wget -S -O /dev/null "$latest_url" 2>&1 || true)"
    debug "latest tag probe (wget): $(printf '%s' "$out" | tail -n 3)"
    effective="$(printf '%s' "$out" | awk 'tolower($1)=="location:"{print $2}' | tail -n 1 | tr -d '\r' || true)"
  fi

  if [[ -n "$effective" ]]; then
    local tag_from_redirect
    tag_from_redirect="${effective##*/}"
    if [[ "$tag_from_redirect" == v* ]]; then
      printf "%s" "$tag_from_redirect"
      return 0
    fi
  fi

  # Fallback: GitHub API
  local url="https://api.github.com/repos/${REPO}/releases/latest"
  local tmp
  tmp="$(mktemp)"
  rm -f "$tmp" || true

  if ! try_download "$url" "$tmp"; then
    debug "github api latest failed: url=$url err='${LAST_TRY_ERROR:-}'"
    rm -f "$tmp" || true
    return 1
  fi

  local tag=""
  if command -v python3 >/dev/null 2>&1; then
    tag="$(python3 - <<'PY' <"$tmp" 2>/dev/null || true
import json,sys
try:
  print(json.load(sys.stdin).get('tag_name',''))
except Exception:
  print('')
PY
)"
  else
    tag="$(grep -Eo '"tag_name"\s*:\s*"[^"]+"' "$tmp" | head -n 1 | sed -E 's/.*"([^"]+)"$/\1/')"
  fi

  rm -f "$tmp" || true
  [[ -n "$tag" ]] || return 1
  printf "%s" "$tag"
}

random_port() {
  if command -v shuf >/dev/null 2>&1; then
    shuf -i 20000-59999 -n 1
  else
    echo $(( (RANDOM % 40000) + 20000 ))
  fi
}

random_psk() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 32 | tr -d '\n' | tr '+/' '-_' | tr -d '='
    return
  fi

  if command -v python3 >/dev/null 2>&1; then
    python3 - <<'PY'
import secrets
import string
alphabet = string.ascii_letters + string.digits
print(''.join(secrets.choice(alphabet) for _ in range(48)))
PY
    return
  fi

  if command -v base64 >/dev/null 2>&1; then
    head -c 32 /dev/urandom | base64 | tr -d '\n' | tr '+/' '-_' | tr -d '='
    return
  fi

  head -c 96 /dev/urandom | tr -dc 'A-Za-z0-9' | head -c 48
}

detect_arch() {
  # Map uname -m to a small set of common Go/GitHub release arch names.
  local m
  m="$(uname -m)"
  case "$m" in
    x86_64|amd64)
      echo "amd64" ;;
    aarch64|arm64)
      echo "arm64" ;;
    armv7l|armv7)
      echo "armv7" ;;
    armv6l|armv6)
      echo "armv6" ;;
    i386|i686)
      echo "386" ;;
    *)
      echo "${m}" ;;
  esac
}

install_from_release() {
  [[ "$OPEN_SNELL_RELEASE" != "0" ]] || return 1

  local arch asset out url release_tag
  arch="$(detect_arch)"

  release_tag="$OPEN_SNELL_TAG"

  out="$(mktemp)"

  # Candidate asset names.
  # We support either a raw snell-server binary or a tar.gz bundle containing it.
  local -a candidates
  if [[ -n "$OPEN_SNELL_ASSET" ]]; then
    candidates=("$OPEN_SNELL_ASSET")
  else
    candidates=(
      # Preferred: stable asset names that work with releases/latest/download
      "open-snell-linux-${arch}.tar.gz"
      "open-snell-linux-${arch}.tgz"
      "snell-server_linux_${arch}"
      "snell_linux_${arch}.tar.gz"
      "snell_linux_${arch}.tgz"
    )

    # If a tag is specified (or we can determine one), also try versioned asset names.
    if [[ -z "$release_tag" ]]; then
      release_tag="$(fetch_latest_tag || true)"
    fi
    if [[ -n "$release_tag" ]]; then
      candidates=(
        "open-snell-${release_tag}-linux-${arch}.tar.gz"
        "open-snell-${release_tag}-linux-${arch}.tgz"
        "snell-${release_tag}-linux-${arch}.tar.gz"
        "snell-${release_tag}-linux-${arch}.tgz"
        "${candidates[@]}"
      )
    fi
    if [[ -z "$release_tag" ]]; then
      log "Note: could not determine latest release tag."
      log "- If your release assets are versioned like open-snell-vX.Y.Z-..., set OPEN_SNELL_TAG=vX.Y.Z"
      log "- Or ensure the release also uploads stable assets like open-snell-linux-${arch}.tar.gz"
      log "- For more diagnostics: OPEN_SNELL_DEBUG=1"
    fi
  fi

  local cand
  for cand in "${candidates[@]}"; do
    if [[ -n "$release_tag" && "$cand" == *"${release_tag}"* ]]; then
      url="https://github.com/${REPO}/releases/download/${release_tag}/${cand}"
    else
      url="https://github.com/${REPO}/releases/latest/download/${cand}"
    fi

    rm -f "$out" || true
    log "Attempting release install: ${url}"
    if ! try_download "$url" "$out"; then
      continue
    fi
    if [[ ! -s "$out" ]]; then
      continue
    fi

    # If it's a tar.gz, try to extract snell-server from it.
    if command -v tar >/dev/null 2>&1 && tar -tzf "$out" >/dev/null 2>&1; then
      need_cmd tar
      local tmpd
      tmpd="$(mktemp -d)"
      tar -xzf "$out" -C "$tmpd"

      local extracted
      extracted="$(find "$tmpd" -type f -name 'snell-server' -o -name 'snell-server_linux_*' | head -n 1)"
      if [[ -z "$extracted" ]]; then
        # As a fallback, try to find any executable named snell-server (some packs nest paths).
        extracted="$(find "$tmpd" -type f -name 'snell-server' | head -n 1)"
      fi
      if [[ -z "$extracted" ]]; then
        rm -rf "$tmpd" || true
        continue
      fi

      log "Installing binary to ${BIN_INSTALL_PATH}"
      install -D -m 0755 "$extracted" "$BIN_INSTALL_PATH"
      rm -rf "$tmpd" || true
      rm -f "$out" || true
      return 0
    fi

    # Otherwise treat as a raw binary.
    log "Installing binary to ${BIN_INSTALL_PATH}"
    install -D -m 0755 "$out" "$BIN_INSTALL_PATH"
    rm -f "$out" || true
    return 0
  done

  rm -f "$out" || true
  log "Release asset not available, falling back to source build"
  if [[ -n "${LAST_TRY_URL:-}" ]]; then
    log "Last attempted URL: ${LAST_TRY_URL}"
  fi
  if [[ -n "${LAST_TRY_ERROR:-}" ]]; then
    log "Last error: ${LAST_TRY_ERROR}"
  fi
  return 1
}

install_from_source() {
  need_cmd tar
  need_cmd go

  local tmp
  tmp="$(mktemp -d)"
  # set -u safe: expand tmp now so the EXIT trap doesn't reference a local var
  trap "rm -rf '$tmp'" EXIT

  local tgz="$tmp/opensnell.tar.gz"
  local url="https://github.com/${REPO}/archive/refs/heads/${REF}.tar.gz"

  log "Downloading ${url}"
  download "$url" "$tgz"

  log "Extracting"
  tar -xzf "$tgz" -C "$tmp"

  local src_dir
  src_dir="$(find "$tmp" -maxdepth 1 -type d -name 'opensnell-*' | head -n 1)"
  [[ -n "$src_dir" ]] || die "failed to find extracted source directory"

  log "Building snell-server"
  local built_bin
  if command -v make >/dev/null 2>&1; then
    (cd "$src_dir" && make server)
    built_bin="$src_dir/build/snell-server"
  else
    (cd "$src_dir" && CGO_ENABLED=0 go build -trimpath -o "$src_dir/snell-server" ./cmd/snell-server)
    built_bin="$src_dir/snell-server"
  fi

  [[ -f "$built_bin" ]] || die "build succeeded but binary not found: $built_bin"

  log "Installing binary to ${BIN_INSTALL_PATH}"
  install -D -m 0755 "$built_bin" "$BIN_INSTALL_PATH"
}

main() {
  [[ "$(id -u)" -eq 0 ]] || die "please run as root (e.g. sudo bash install.sh)"

  # If DESTDIR is set, default to skipping systemctl unless explicitly forced.
  if [[ -n "$OPEN_SNELL_DESTDIR" && -z "$OPEN_SNELL_SKIP_SYSTEMD" ]]; then
    OPEN_SNELL_SKIP_SYSTEMD="1"
  fi

  if [[ "$OPEN_SNELL_SKIP_SYSTEMD" != "1" ]]; then
    need_cmd systemctl
  fi

  if ! install_from_release; then
    install_from_source
  fi

  log "Ensuring config directory ${CONFIG_DIR}"
  install -d -m 0755 "$CONFIG_DIR"

  if [[ ! -f "$CONFIG_PATH" ]]; then
    local port psk
    port="$(random_port)"
    psk="$(random_psk)"

    log "Writing new config ${CONFIG_PATH}"
    cat >"$CONFIG_PATH" <<EOF
[snell-server]
listen = 0.0.0.0:${port}
psk = ${psk}
obfs = http
verbose = true
EOF
    chmod 0640 "$CONFIG_PATH"

    log "Generated defaults: listen=0.0.0.0:${port}"
    log "Generated defaults: psk=${psk}"
  else
    log "Config already exists: ${CONFIG_PATH} (leaving as-is)"
  fi

  log "Installing systemd unit ${UNIT_PATH}"
  install -d -m 0755 "$(dirname "$UNIT_PATH")"
  cat >"$UNIT_PATH" <<EOF
[Unit]
Description=OpenSnell snell-server
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
ExecStart=${TARGET_BIN_PATH} -c ${TARGET_CONFIG_PATH}
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF

  if [[ "$OPEN_SNELL_SKIP_SYSTEMD" != "1" ]]; then
    log "Enabling and starting snell-server.service"
    systemctl daemon-reload
    systemctl enable --now snell-server.service
  else
    log "Skipping systemd operations (OPEN_SNELL_SKIP_SYSTEMD=1)"
  fi

  log "Done. Useful commands:"
  log "  systemctl status snell-server.service"
  log "  journalctl -u snell-server.service -f"
}

main "$@"
