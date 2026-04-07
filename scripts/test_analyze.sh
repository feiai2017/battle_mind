#!/usr/bin/env bash

set -uo pipefail

BASE_URL="${BASE_URL:-}"
PORT="${PORT:-8080}"
if [[ -z "${BASE_URL}" ]]; then
  BASE_URL="http://localhost:${PORT}"
fi

ANALYZE_URL="${BASE_URL}/analyze"
HEALTH_URL="${BASE_URL}/health"
FORCE_FAIL="${FORCE_FAIL:-0}"

total_cases=0
passed_cases=0
failed_cases=0

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1"
    exit 1
  fi
}

print_summary() {
  echo
  echo "done: ${total_cases} cases, ${passed_cases} passed, ${failed_cases} failed"
}

health_check() {
  local body_file status
  body_file="$(mktemp)"
  if ! status="$(curl -sS -o "${body_file}" -w "%{http_code}" "${HEALTH_URL}")"; then
    rm -f "${body_file}"
    echo "server not reachable: ${HEALTH_URL}"
    echo "please start service first"
    exit 1
  fi

  if [[ "${status}" != "200" ]] || ! grep -q '"ok"[[:space:]]*:[[:space:]]*true' "${body_file}"; then
    echo "server health check failed: ${HEALTH_URL}"
    echo "status=${status}"
    echo "body=$(tr '\r\n' ' ' < "${body_file}" | cut -c1-200)"
    rm -f "${body_file}"
    exit 1
  fi

  rm -f "${body_file}"
}

run_case() {
  local name="$1"
  local request_file="$2"
  local expected_status="$3"
  local expected_keyword="$4"
  local extra_header="${5:-}"

  total_cases=$((total_cases + 1))
  local body_file status curl_exit summary result="PASS"
  body_file="$(mktemp)"

  echo "[${total_cases}/5] ${name}"
  echo "target=${ANALYZE_URL}"

  if [[ ! -f "${request_file}" ]]; then
    echo "status=missing-file"
    echo "result=FAIL"
    echo "reason=request file not found: ${request_file}"
    echo
    failed_cases=$((failed_cases + 1))
    rm -f "${body_file}"
    return
  fi

  local curl_args=(
    -sS
    -o "${body_file}"
    -w "%{http_code}"
    -X POST "${ANALYZE_URL}"
    -H "Content-Type: application/json"
    --data "@${request_file}"
  )
  if [[ -n "${extra_header}" ]]; then
    curl_args+=(-H "${extra_header}")
  fi

  status="$(curl "${curl_args[@]}")"
  curl_exit=$?
  if [[ ${curl_exit} -ne 0 ]]; then
    echo "status=curl-error"
    echo "result=FAIL"
    echo "reason=curl exit code ${curl_exit}"
    echo
    failed_cases=$((failed_cases + 1))
    rm -f "${body_file}"
    return
  fi

  summary="$(tr '\r\n' ' ' < "${body_file}" | sed 's/[[:space:]]\+/ /g' | cut -c1-220)"
  echo "status=${status}"

  if [[ "${status}" != "${expected_status}" ]]; then
    result="FAIL"
    echo "reason=unexpected status, expected ${expected_status}"
  elif [[ -n "${expected_keyword}" ]] && ! grep -q "${expected_keyword}" "${body_file}"; then
    result="FAIL"
    echo "reason=response missing keyword: ${expected_keyword}"
  fi

  if [[ "${result}" == "PASS" ]]; then
    passed_cases=$((passed_cases + 1))
  else
    failed_cases=$((failed_cases + 1))
  fi

  echo "result=${result}"
  echo "response=${summary}"
  echo
  rm -f "${body_file}"
}

require_command curl
health_check

normal_status="200"
if [[ "${FORCE_FAIL}" == "1" ]]; then
  normal_status="201"
fi

run_case "normal" "testdata/request_normal.json" "${normal_status}" "\"summary\""
run_case "short-log" "testdata/request_short_log.json" "200" "\"summary\""
run_case "long-log" "testdata/request_long_log.json" "400" "LOG_TOO_LONG"
run_case "invalid-input" "testdata/request_invalid.json" "400" "EMPTY_LOG_TEXT"
run_case "timeout-sim" "testdata/request_timeout_sim.json" "502" "ANALYZE_FAILED" "X-Debug-Simulate-Timeout: 1"

print_summary

if [[ ${failed_cases} -gt 0 ]]; then
  exit 1
fi

exit 0
