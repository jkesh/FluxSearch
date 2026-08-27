#!/usr/bin/env bash
# 从 infra.env / deploy.env 加载环境变量（不覆盖已 export 的值）
set -euo pipefail

_load_env_file() {
  local file="$1"
  if [[ ! -f "$file" ]]; then
    return 1
  fi
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%%#*}"
    line="$(echo "$line" | xargs)"
    [[ -z "$line" ]] && continue
    if [[ "$line" != *=* ]]; then
      continue
    fi
    local key="${line%%=*}"
    local val="${line#*=}"
    key="$(echo "$key" | xargs)"
    val="$(echo "$val" | xargs)"
    if [[ -z "${key}" ]]; then
      continue
    fi
    if [[ -z "${!key:-}" ]]; then
      export "${key}=${val}"
    fi
  done <"$file"
  return 0
}

fluxsearch_load_config() {
  local root="${1:-.}"
  _load_env_file "${root}/config/local/infra.env" || true
  _load_env_file "${root}/config/local/deploy.env" || true
}

fluxsearch_init_config() {
  local root="${1:-.}"
  mkdir -p "${root}/config/local"
  if [[ ! -f "${root}/config/local/infra.env" ]]; then
    cp "${root}/config/infra.example.env" "${root}/config/local/infra.env"
    echo "已创建 config/local/infra.env（请编辑连接信息）"
  fi
  if [[ ! -f "${root}/config/local/deploy.env" ]]; then
    cp "${root}/config/deploy.example.env" "${root}/config/local/deploy.env"
    echo "已创建 config/local/deploy.env"
  fi
  if [[ ! -f "${root}/config/local/app.settings.json" ]]; then
    cp "${root}/config/app.settings.example.json" "${root}/config/local/app.settings.json"
    echo "已创建 config/local/app.settings.json"
  fi
}

fluxsearch_flagembedding_url() {
  local host="${FLUXSEARCH_FLAGEMBEDDING_HOST:-127.0.0.1}"
  local port="${FLUXSEARCH_FLAGEMBEDDING_PORT:-8091}"
  echo "http://${host}:${port}/v1"
}
