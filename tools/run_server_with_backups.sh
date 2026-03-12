#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  tools/run_server_with_backups.sh \
    --config-path ./config/standalone-config.yaml \
    --genesis-path ./config/standalone-genesis.yaml \
    --backup-root /tmp/iotex-backups \
    --backup-path /var/data \
    --backup-path /var/log/systemlog.db \
    --height 34000000 \
    --height 35000000 \
    [--server-bin ./bin/server] \
    [--readtip-bin ./bin/readtip] \
    [--secret-path ./config/secret.yaml] \
    [--plugin gateway] \
    [--backup-cmd 'tar -C / -czf "$BACKUP_DEST/data.tgz" var/data'] \
    [-- auto-pass-through-server-args]

Behavior:
  - Starts the server with --stop-at-height for each requested height.
  - Waits for a clean exit.
  - Verifies the on-disk tip equals the requested height.
  - Runs a custom backup command, or copies every --backup-path into
    $BACKUP_ROOT/height-<target>/.

Environment passed to --backup-cmd:
  TARGET_HEIGHT, BACKUP_DEST, CONFIG_PATH, GENESIS_PATH, SECRET_PATH
EOF
}

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
server_bin="${repo_root}/bin/server"
readtip_bin="${repo_root}/bin/readtip"
config_path=""
genesis_path=""
secret_path=""
backup_root=""
backup_cmd=""
declare -a heights=()
declare -a backup_paths=()
declare -a plugins=()
declare -a extra_server_args=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config-path)
      config_path="$2"
      shift 2
      ;;
    --genesis-path)
      genesis_path="$2"
      shift 2
      ;;
    --secret-path)
      secret_path="$2"
      shift 2
      ;;
    --backup-root)
      backup_root="$2"
      shift 2
      ;;
    --backup-path)
      backup_paths+=("$2")
      shift 2
      ;;
    --height)
      heights+=("$2")
      shift 2
      ;;
    --plugin)
      plugins+=("$2")
      shift 2
      ;;
    --server-bin)
      server_bin="$2"
      shift 2
      ;;
    --readtip-bin)
      readtip_bin="$2"
      shift 2
      ;;
    --backup-cmd)
      backup_cmd="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    --)
      shift
      extra_server_args=("$@")
      break
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$config_path" || -z "$genesis_path" || -z "$backup_root" || ${#heights[@]} -eq 0 ]]; then
  echo "Missing required arguments." >&2
  usage >&2
  exit 1
fi

if [[ -z "$backup_cmd" && ${#backup_paths[@]} -eq 0 ]]; then
  echo "Provide at least one --backup-path or a --backup-cmd." >&2
  exit 1
fi

cd "$repo_root"

if [[ ! -x "$server_bin" || ! -x "$readtip_bin" ]]; then
  echo "Building required binaries..."
  make build build-readtip
fi

mkdir -p "$backup_root"

read_tip() {
  local cmd=("$readtip_bin" "-config-path=$config_path")
  local output=""
  local height=""
  if [[ -n "$secret_path" ]]; then
    cmd+=("-secret-path=$secret_path")
  fi
  if ! output="$("${cmd[@]}" 2>&1)"; then
    if grep -qE 'failed to load state db|failed to read state db' <<<"$output"; then
      echo ""
      return 0
    fi
    printf '%s\n' "$output" >&2
    return 1
  fi
  height="$(printf '%s\n' "$output" | grep -Eo '[0-9]+' | tail -n 1 || true)"
  if [[ -z "$height" ]]; then
    printf 'Unable to parse tip height from readtip output:\n%s\n' "$output" >&2
    return 1
  fi
  printf '%s' "$height"
}

copy_backup_paths() {
  local dest="$1"
  mkdir -p "$dest"
  for path in "${backup_paths[@]}"; do
    if [[ ! -e "$path" ]]; then
      echo "Backup path does not exist: $path" >&2
      exit 1
    fi
    cp -a "$path" "$dest/"
  done
}

for target in "${heights[@]}"; do
  if ! [[ "$target" =~ ^[0-9]+$ ]]; then
    echo "Invalid height: $target" >&2
    exit 1
  fi

  current_tip="$(read_tip)"
  if [[ -n "$current_tip" && "$current_tip" =~ ^[0-9]+$ ]]; then
    if (( current_tip > target )); then
      echo "Current tip ${current_tip} is already past requested height ${target}." >&2
      exit 1
    fi
  fi

  if [[ "$current_tip" != "$target" ]]; then
    cmd=(
      "$server_bin"
      "-config-path=$config_path"
      "-genesis-path=$genesis_path"
      "-stop-at-height=$target"
    )
    if [[ -n "$secret_path" ]]; then
      cmd+=("-secret-path=$secret_path")
    fi
    if ((${#plugins[@]} > 0)); then
      for plugin in "${plugins[@]}"; do
        cmd+=("-plugin=$plugin")
      done
    fi
    if ((${#extra_server_args[@]} > 0)); then
      cmd+=("${extra_server_args[@]}")
    fi

    echo "Starting server until height ${target}..."
    "${cmd[@]}"
  else
    echo "Height ${target} already reached on disk, skipping restart."
  fi

  final_tip="$(read_tip)"
  if [[ "$final_tip" != "$target" ]]; then
    echo "Expected tip ${target}, got ${final_tip}." >&2
    exit 1
  fi

  backup_dest="${backup_root}/height-${target}"
  rm -rf "$backup_dest"
  mkdir -p "$backup_dest"

  if [[ -n "$backup_cmd" ]]; then
    echo "Running custom backup command for height ${target}..."
    TARGET_HEIGHT="$target" \
    BACKUP_DEST="$backup_dest" \
    CONFIG_PATH="$config_path" \
    GENESIS_PATH="$genesis_path" \
    SECRET_PATH="$secret_path" \
    bash -lc "$backup_cmd"
  else
    echo "Copying backup paths for height ${target}..."
    copy_backup_paths "$backup_dest"
  fi

  echo "Backup completed for height ${target}: ${backup_dest}"
done
