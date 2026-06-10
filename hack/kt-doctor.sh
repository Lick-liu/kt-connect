#!/usr/bin/env bash

set -uo pipefail

SCRIPT_NAME="$(basename "$0")"

NAMESPACE="${KT_NAMESPACE:-}"
SERVICE="${KT_SERVICE:-tea-shop-merchant-be-biz}"
LOCAL_PORT="${KT_LOCAL_PORT:-81}"
REMOTE_PORT="${KT_REMOTE_PORT:-8080}"
VERSION_MARK="${KT_VERSION_MARK:-zodance-version:jz2}"
CONNECT_UNIT="${KT_CONNECT_UNIT:-ktctl-connect.service}"
MESH_UNIT="${KT_MESH_UNIT:-ktctl-2b.service}"
OUTBOUND_SERVICE="${KT_OUTBOUND_SERVICE:-tea-shop-base-biz}"
OUTBOUND_PORT="${KT_OUTBOUND_PORT:-8080}"
LOCAL_HEALTH_PATH="${KT_LOCAL_HEALTH_PATH:-/actuator/health}"
INGRESS_HEALTH_PATH="${KT_INGRESS_HEALTH_PATH:-/actuator/health/readiness}"
OUTBOUND_PATH="${KT_OUTBOUND_PATH:-/corp_user/info/jz}"
OUTBOUND_READINESS_PATH="${KT_OUTBOUND_READINESS_PATH:-/actuator/health/readiness}"
HTTP_TIMEOUT="${KT_HTTP_TIMEOUT:-8}"
WAIT_TIMEOUT="${KT_WAIT_TIMEOUT:-60}"
FIX=0
RESTART_CONNECT=0
RESTART_MESH=0
RECOVER_SERVICE=0
FORCE_RECOVER_SERVICE=0
CLEAN_LOCAL_PIDS=0
SKIP_NETWORK_TESTS=0
NO_COLOR=0
NO_LOG_FILE=0
LOG_FILE="${KT_DOCTOR_LOG_FILE:-}"

STATUS_CONNECT="skip"
STATUS_MESH_UNIT="skip"
STATUS_LOCAL_PORT="skip"
STATUS_TARGET_SERVICE="skip"
STATUS_ROUTER="skip"
STATUS_MESH_RESOURCE="skip"
STATUS_MESH_TUNNEL="skip"
STATUS_INGRESS="skip"
STATUS_OUTBOUND="skip"
STATUS_PID_FILES="skip"

REASON_CONNECT=""
REASON_MESH_UNIT=""
REASON_LOCAL_PORT=""
REASON_TARGET_SERVICE=""
REASON_ROUTER=""
REASON_MESH_RESOURCE=""
REASON_MESH_TUNNEL=""
REASON_INGRESS=""
REASON_OUTBOUND=""
REASON_PID_FILES=""

TARGET_POINTS_TO_ROUTER=0
ORPHAN_ROUTER=0
TARGET_ENDPOINT_COUNT=0
ROUTER_RUNNING_COUNT=0
LOCAL_PORT_LISTENING=0
CONNECT_RESTARTED=0
MESH_RESTARTED=0
SERVICE_EXISTS=0
ORIGINAL_SELECTOR=""
CURRENT_SELECTOR_JSON="{}"
MESH_NAME=""
ROUTER_NAME=""
STUNTMAN_NAME=""
CURRENT_USER="$(id -un 2>/dev/null || whoami 2>/dev/null || printf unknown)"
TMP_DIR=""

usage() {
  cat <<'EOF'
Usage:
  kt-doctor.sh [options]

Default mode is read-only diagnosis. Pass --fix to apply safe local repairs.

Options:
  -n, --namespace NAME        Kubernetes namespace. Defaults to current context namespace or "dev".
  -s, --service NAME          Target service to mesh. Default: tea-shop-merchant-be-biz.
      --local-port PORT       Local application port. Default: 81.
      --remote-port PORT      Remote service port. Default: 8080.
      --version-mark K:V      Mesh header/version mark. Default: zodance-version:jz2.
      --connect-unit NAME     systemd --user ktctl connect unit. Default: ktctl-connect.service.
      --mesh-unit NAME        systemd --user ktctl mesh unit. Default: ktctl-2b.service.
      --outbound-service NAME Service used to test ktctl connect. Default: tea-shop-base-biz.
      --outbound-port PORT    Outbound service port. Default: 8080.
      --local-health-path P   Local app path checked by mesh tunnel. Default: /actuator/health.
      --ingress-path P        In-cluster header route check path. Default: /actuator/health/readiness.
      --outbound-path P       Local outbound check path. Default: /corp_user/info/jz.
      --http-timeout SECONDS  Timeout for curl/wget checks. Default: 8.
      --wait-timeout SECONDS  Timeout for restart wait loops. Default: 60.
      --skip-network-tests    Skip curl/kubectl exec network probes.
      --log-file PATH         Append detailed logs to PATH. Default: ~/.kt/doctor/kt-doctor-*.log.
      --no-log-file           Print only to terminal.
      --no-color              Disable terminal colors.

Repair options:
      --fix                   Apply safe fixes detected by diagnosis.
      --restart-connect       Restart only the local connect systemd unit.
      --restart-mesh          Restart only the local mesh systemd unit.
      --recover-service       Recover target Service selector only when it is orphaned to an empty/missing router.
      --force-recover-service Recover target Service selector even if router is not detected as orphaned.
      --clean-local-pids      Remove stale ~/.kt/pid files for dead local PIDs.

Safety:
  This script never runs broad "ktctl clean" and never deletes kt resources owned by other users.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -n|--namespace)
      NAMESPACE="${2:-}"
      shift 2
      ;;
    -s|--service)
      SERVICE="${2:-}"
      shift 2
      ;;
    --local-port)
      LOCAL_PORT="${2:-}"
      shift 2
      ;;
    --remote-port)
      REMOTE_PORT="${2:-}"
      shift 2
      ;;
    --version-mark)
      VERSION_MARK="${2:-}"
      shift 2
      ;;
    --connect-unit)
      CONNECT_UNIT="${2:-}"
      shift 2
      ;;
    --mesh-unit)
      MESH_UNIT="${2:-}"
      shift 2
      ;;
    --outbound-service)
      OUTBOUND_SERVICE="${2:-}"
      shift 2
      ;;
    --outbound-port)
      OUTBOUND_PORT="${2:-}"
      shift 2
      ;;
    --local-health-path)
      LOCAL_HEALTH_PATH="${2:-}"
      shift 2
      ;;
    --ingress-path)
      INGRESS_HEALTH_PATH="${2:-}"
      shift 2
      ;;
    --outbound-path)
      OUTBOUND_PATH="${2:-}"
      shift 2
      ;;
    --http-timeout)
      HTTP_TIMEOUT="${2:-}"
      shift 2
      ;;
    --wait-timeout)
      WAIT_TIMEOUT="${2:-}"
      shift 2
      ;;
    --skip-network-tests)
      SKIP_NETWORK_TESTS=1
      shift
      ;;
    --log-file)
      LOG_FILE="${2:-}"
      shift 2
      ;;
    --no-log-file)
      NO_LOG_FILE=1
      shift
      ;;
    --no-color)
      NO_COLOR=1
      shift
      ;;
    --fix)
      FIX=1
      shift
      ;;
    --restart-connect)
      FIX=1
      RESTART_CONNECT=1
      shift
      ;;
    --restart-mesh)
      FIX=1
      RESTART_MESH=1
      shift
      ;;
    --recover-service)
      FIX=1
      RECOVER_SERVICE=1
      shift
      ;;
    --force-recover-service)
      FIX=1
      RECOVER_SERVICE=1
      FORCE_RECOVER_SERVICE=1
      shift
      ;;
    --clean-local-pids)
      FIX=1
      CLEAN_LOCAL_PIDS=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'Unknown option: %s\n\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -t 1 && "$NO_COLOR" -eq 0 ]]; then
  C_RESET=$'\033[0m'
  C_RED=$'\033[31m'
  C_GREEN=$'\033[32m'
  C_YELLOW=$'\033[33m'
  C_BLUE=$'\033[34m'
else
  C_RESET=""
  C_RED=""
  C_GREEN=""
  C_YELLOW=""
  C_BLUE=""
fi

ts() {
  date '+%Y-%m-%d %H:%M:%S'
}

log() {
  local level="$1"
  shift
  local color="$C_BLUE"
  case "$level" in
    PASS) color="$C_GREEN" ;;
    WARN) color="$C_YELLOW" ;;
    FAIL|ERROR) color="$C_RED" ;;
    RUN) color="$C_BLUE" ;;
  esac
  printf '%s[%s] [%s]%s %s\n' "$color" "$(ts)" "$level" "$C_RESET" "$*"
}

section() {
  printf '\n'
  log INFO "== $* =="
}

die() {
  log ERROR "$*"
  exit 2
}

is_uint() {
  [[ "$1" =~ ^[0-9]+$ ]]
}

quote_cmd() {
  local first=1
  local arg
  for arg in "$@"; do
    if [[ "$first" -eq 0 ]]; then
      printf ' '
    fi
    printf '%q' "$arg"
    first=0
  done
}

print_snippet() {
  local text="$1"
  local max_lines="${2:-40}"
  if [[ -z "$text" ]]; then
    return 0
  fi
  printf '%s\n' "$text" | sed -n "1,${max_lines}p" | sed 's/^/    /'
  local line_count
  line_count="$(printf '%s\n' "$text" | wc -l | tr -d ' ')"
  if [[ "$line_count" -gt "$max_lines" ]]; then
    printf '    ... (%s more lines)\n' "$((line_count - max_lines))"
  fi
}

run_cmd() {
  log RUN "$(quote_cmd "$@")"
  "$@"
  local code=$?
  if [[ "$code" -eq 0 ]]; then
    log PASS "exit $code"
  else
    log FAIL "exit $code"
  fi
  return "$code"
}

capture_cmd() {
  local output
  log RUN "$(quote_cmd "$@")"
  output="$("$@" 2>&1)"
  local code=$?
  print_snippet "$output" 80
  if [[ "$code" -eq 0 ]]; then
    log PASS "exit $code"
  else
    log FAIL "exit $code"
  fi
  CAPTURE_OUTPUT="$output"
  return "$code"
}

normalize_path() {
  local path="$1"
  if [[ -z "$path" ]]; then
    printf '/'
    return 0
  fi
  if [[ "$path" != /* ]]; then
    printf '/%s' "$path"
  else
    printf '%s' "$path"
  fi
}

setup_logging() {
  if [[ "$NO_LOG_FILE" -eq 1 ]]; then
    return 0
  fi
  if [[ -z "$LOG_FILE" ]]; then
    LOG_FILE="${HOME}/.kt/doctor/kt-doctor-$(date '+%Y%m%d-%H%M%S').log"
  fi
  mkdir -p "$(dirname "$LOG_FILE")" || die "Cannot create log directory for $LOG_FILE"
  touch "$LOG_FILE" || die "Cannot write log file $LOG_FILE"
  exec > >(tee -a "$LOG_FILE") 2>&1
}

validate_inputs() {
  [[ -n "$SERVICE" ]] || die "--service cannot be empty"
  [[ -n "$VERSION_MARK" ]] || die "--version-mark cannot be empty"
  [[ "$VERSION_MARK" == *:* ]] || die "--version-mark must look like key:value, got $VERSION_MARK"
  is_uint "$LOCAL_PORT" || die "--local-port must be numeric, got $LOCAL_PORT"
  is_uint "$REMOTE_PORT" || die "--remote-port must be numeric, got $REMOTE_PORT"
  is_uint "$OUTBOUND_PORT" || die "--outbound-port must be numeric, got $OUTBOUND_PORT"
  is_uint "$HTTP_TIMEOUT" || die "--http-timeout must be numeric, got $HTTP_TIMEOUT"
  is_uint "$WAIT_TIMEOUT" || die "--wait-timeout must be numeric, got $WAIT_TIMEOUT"
  LOCAL_HEALTH_PATH="$(normalize_path "$LOCAL_HEALTH_PATH")"
  INGRESS_HEALTH_PATH="$(normalize_path "$INGRESS_HEALTH_PATH")"
  OUTBOUND_PATH="$(normalize_path "$OUTBOUND_PATH")"
  OUTBOUND_READINESS_PATH="$(normalize_path "$OUTBOUND_READINESS_PATH")"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "Required command not found: $1"
}

load_defaults() {
  if [[ -z "$NAMESPACE" ]]; then
    NAMESPACE="$(kubectl config view --minify --output 'jsonpath={..namespace}' 2>/dev/null || true)"
  fi
  if [[ -z "$NAMESPACE" ]]; then
    NAMESPACE="dev"
  fi

  local header_value="${VERSION_MARK#*:}"
  MESH_NAME="${KT_MESH_NAME:-${SERVICE}-kt-mesh-${header_value}}"
  ROUTER_NAME="${KT_ROUTER_NAME:-${SERVICE}-kt-router}"
  STUNTMAN_NAME="${KT_STUNTMAN_NAME:-${SERVICE}-kt-stuntman}"
}

init() {
  setup_logging
  validate_inputs
  require_command kubectl
  require_command jq
  require_command curl
  require_command timeout
  require_command systemctl
  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "$TMP_DIR"' EXIT
  load_defaults
}

status_log() {
  local status="$1"
  shift
  case "$status" in
    pass) log PASS "$*" ;;
    warn) log WARN "$*" ;;
    fail) log FAIL "$*" ;;
    skip) log WARN "$*" ;;
    *) log INFO "$*" ;;
  esac
}

show_context() {
  section "Context"
  local context
  context="$(kubectl config current-context 2>/dev/null || true)"
  log INFO "kube context: ${context:-unknown}"
  log INFO "namespace: $NAMESPACE"
  log INFO "current local user: $CURRENT_USER"
  log INFO "target service: $SERVICE"
  log INFO "mesh name: $MESH_NAME"
  log INFO "router name: $ROUTER_NAME"
  log INFO "stuntman name: $STUNTMAN_NAME"
  log INFO "version mark/header: ${VERSION_MARK%%:*}: ${VERSION_MARK#*:}"
  log INFO "mode: $([[ "$FIX" -eq 1 ]] && printf fix || printf diagnose-only)"
  if [[ "$NO_LOG_FILE" -eq 0 ]]; then
    log INFO "log file: $LOG_FILE"
  fi
}

check_systemd_unit() {
  local unit="$1"
  local status_var="$2"
  local reason_var="$3"
  local active

  active="$(systemctl --user is-active "$unit" 2>/dev/null || true)"
  capture_cmd systemctl --user show "$unit" -p Id -p ActiveState -p SubState -p MainPID -p ExecStart --no-pager || true
  if [[ "$active" == "active" ]]; then
    printf -v "$status_var" '%s' "pass"
    printf -v "$reason_var" '%s' "$unit is active"
    log PASS "$unit is active"
  else
    printf -v "$status_var" '%s' "fail"
    printf -v "$reason_var" '%s' "$unit is $active"
    log FAIL "$unit is $active"
    capture_cmd journalctl --user -u "$unit" -n 30 --no-pager || true
  fi
}

check_systemd() {
  section "Local systemd units"
  check_systemd_unit "$CONNECT_UNIT" STATUS_CONNECT REASON_CONNECT
  check_systemd_unit "$MESH_UNIT" STATUS_MESH_UNIT REASON_MESH_UNIT
}

check_local_port() {
  section "Local app port"
  LOCAL_PORT_LISTENING=0

  if command -v ss >/dev/null 2>&1; then
    local listen
    listen="$(ss -ltn 2>/dev/null | awk -v p=":${LOCAL_PORT}" '$4 ~ p "$" {print}')"
    if [[ -n "$listen" ]]; then
      LOCAL_PORT_LISTENING=1
      log PASS "local port $LOCAL_PORT is listening"
      print_snippet "$listen" 20
    else
      log FAIL "local port $LOCAL_PORT is not listening"
    fi
  else
    log WARN "ss not found; skip TCP listen check"
  fi

  if [[ "$SKIP_NETWORK_TESTS" -eq 1 ]]; then
    STATUS_LOCAL_PORT="skip"
    REASON_LOCAL_PORT="network tests skipped"
    log WARN "skip local HTTP probe"
    return 0
  fi

  local url="http://127.0.0.1:${LOCAL_PORT}${LOCAL_HEALTH_PATH}"
  local output
  log RUN "curl local app: $url"
  output="$(curl -sS -i --max-time "$HTTP_TIMEOUT" "$url" 2>&1)"
  local code=$?
  print_snippet "$output" 30
  if [[ "$code" -eq 0 ]]; then
    STATUS_LOCAL_PORT="pass"
    REASON_LOCAL_PORT="local app replied on $url"
    LOCAL_PORT_LISTENING=1
    log PASS "local app replied"
  else
    STATUS_LOCAL_PORT="fail"
    REASON_LOCAL_PORT="local app did not reply on $url"
    log FAIL "local app probe failed with curl exit $code"
  fi
}

list_kt_resources() {
  section "kt resources in namespace"
  local resources_json="$TMP_DIR/kt-resources.json"
  if ! kubectl -n "$NAMESPACE" get pod,svc,endpoints,cm -l control-by=kt -o json >"$resources_json" 2>"$TMP_DIR/kt-resources.err"; then
    log WARN "cannot list kt resources"
    print_snippet "$(cat "$TMP_DIR/kt-resources.err")" 40
    return 0
  fi

  local count
  count="$(jq '.items | length' "$resources_json")"
  log INFO "kt resource count: $count"
  printf '%-10s %-48s %-18s %-16s %-10s %s\n' "KIND" "NAME" "OWNER" "ROLE" "PHASE" "HEARTBEAT"
  jq -r '
    .items[]
    | [
        .kind,
        .metadata.name,
        (.metadata.annotations["kt-user"] // "-"),
        (.metadata.labels["kt-role"] // "-"),
        (.status.phase // "-"),
        (.metadata.annotations["kt-last-heart-beat"] // "-")
      ]
    | @tsv
  ' "$resources_json" | while IFS=$'\t' read -r kind name owner role phase heartbeat; do
    printf '%-10s %-48s %-18s %-16s %-10s %s\n' "$kind" "$name" "$owner" "$role" "$phase" "$heartbeat"
    if [[ "$owner" != "-" && "$owner" != "$CURRENT_USER" ]]; then
      log WARN "foreign kt resource detected: $kind/$name owner=$owner"
    fi
  done

  log INFO "raw kt resources:"
  kubectl -n "$NAMESPACE" get pod,svc,endpoints,cm -l control-by=kt -o wide --show-labels || true
}

label_selector_from_json_object() {
  jq -r 'to_entries | map("\(.key)=\(.value)") | join(",")'
}

check_target_service() {
  section "Target service"
  SERVICE_EXISTS=0
  TARGET_POINTS_TO_ROUTER=0
  ORPHAN_ROUTER=0
  TARGET_ENDPOINT_COUNT=0
  ROUTER_RUNNING_COUNT=0
  ORIGINAL_SELECTOR=""

  local svc_json="$TMP_DIR/target-service.json"
  if ! kubectl -n "$NAMESPACE" get svc "$SERVICE" -o json >"$svc_json" 2>"$TMP_DIR/target-service.err"; then
    STATUS_TARGET_SERVICE="fail"
    REASON_TARGET_SERVICE="service $SERVICE does not exist or cannot be read"
    STATUS_ROUTER="skip"
    REASON_ROUTER="target service unavailable"
    log FAIL "$REASON_TARGET_SERVICE"
    print_snippet "$(cat "$TMP_DIR/target-service.err")" 40
    return 0
  fi

  SERVICE_EXISTS=1
  STATUS_TARGET_SERVICE="pass"
  REASON_TARGET_SERVICE="service $SERVICE exists"
  log PASS "$REASON_TARGET_SERVICE"

  local selector_json
  selector_json="$(jq -c '.spec.selector // {}' "$svc_json")"
  CURRENT_SELECTOR_JSON="$selector_json"
  ORIGINAL_SELECTOR="$(jq -r '.metadata.annotations["kt-selector"] // ""' "$svc_json")"
  log INFO "service selector: $selector_json"
  if [[ -n "$ORIGINAL_SELECTOR" ]]; then
    log INFO "kt-selector annotation: $ORIGINAL_SELECTOR"
  else
    log INFO "kt-selector annotation: <empty>"
  fi

  local ep_json="$TMP_DIR/target-endpoints.json"
  if kubectl -n "$NAMESPACE" get endpoints "$SERVICE" -o json >"$ep_json" 2>/dev/null; then
    TARGET_ENDPOINT_COUNT="$(jq '[.subsets[]?.addresses[]?] | length' "$ep_json")"
    log INFO "target service endpoint address count: $TARGET_ENDPOINT_COUNT"
    jq -r '.subsets[]?.addresses[]?.ip' "$ep_json" | sed 's/^/    endpoint: /' || true
  else
    log WARN "cannot read endpoints/$SERVICE"
  fi

  if jq -e '.spec.selector["kt-role"] == "router"' "$svc_json" >/dev/null; then
    TARGET_POINTS_TO_ROUTER=1
    local router_selector
    router_selector="$(jq -c '.spec.selector // {}' "$svc_json" | label_selector_from_json_object)"
    log INFO "service currently points to kt router selector: $router_selector"
    if [[ -n "$router_selector" ]]; then
      local router_pods_json="$TMP_DIR/router-pods.json"
      if kubectl -n "$NAMESPACE" get pod -l "$router_selector" -o json >"$router_pods_json" 2>/dev/null; then
        ROUTER_RUNNING_COUNT="$(jq '[.items[] | select(.status.phase == "Running")] | length' "$router_pods_json")"
        log INFO "running router pods for target selector: $ROUTER_RUNNING_COUNT"
        jq -r '.items[] | "    pod: \(.metadata.name) phase=\(.status.phase) owner=\(.metadata.annotations["kt-user"] // "-")"' "$router_pods_json" || true
      else
        log WARN "cannot list router pods by selector $router_selector"
      fi
    fi

    if [[ "$TARGET_ENDPOINT_COUNT" -eq 0 || "$ROUTER_RUNNING_COUNT" -eq 0 ]]; then
      ORPHAN_ROUTER=1
      STATUS_ROUTER="fail"
      REASON_ROUTER="service points to router but router endpoints/pods are missing"
      log FAIL "$REASON_ROUTER"
    else
      STATUS_ROUTER="pass"
      REASON_ROUTER="router has running pod and endpoints"
      log PASS "$REASON_ROUTER"
    fi
  else
    STATUS_ROUTER="pass"
    REASON_ROUTER="service does not point to kt router"
    log PASS "$REASON_ROUTER"
  fi
}

check_mesh_resources() {
  section "Mesh resources"
  local pod_json="$TMP_DIR/mesh-pod.json"
  local svc_json="$TMP_DIR/mesh-service.json"
  local ep_json="$TMP_DIR/mesh-endpoints.json"
  local failed=0

  if kubectl -n "$NAMESPACE" get pod "$MESH_NAME" -o json >"$pod_json" 2>"$TMP_DIR/mesh-pod.err"; then
    local phase owner heartbeat
    phase="$(jq -r '.status.phase // "-"' "$pod_json")"
    owner="$(jq -r '.metadata.annotations["kt-user"] // "-"' "$pod_json")"
    heartbeat="$(jq -r '.metadata.annotations["kt-last-heart-beat"] // "-"' "$pod_json")"
    log INFO "mesh pod phase=$phase owner=$owner heartbeat=$heartbeat"
    if [[ "$owner" != "-" && "$owner" != "$CURRENT_USER" ]]; then
      log WARN "mesh pod owner is $owner, not current user $CURRENT_USER"
    fi
    if [[ "$phase" != "Running" ]]; then
      failed=1
      log FAIL "mesh pod is not Running"
    else
      log PASS "mesh pod is Running"
    fi
  else
    failed=1
    log FAIL "mesh pod $MESH_NAME is missing"
    print_snippet "$(cat "$TMP_DIR/mesh-pod.err")" 20
  fi

  if kubectl -n "$NAMESPACE" get svc "$MESH_NAME" -o json >"$svc_json" 2>"$TMP_DIR/mesh-service.err"; then
    log PASS "mesh service $MESH_NAME exists"
    log INFO "mesh service selector: $(jq -c '.spec.selector // {}' "$svc_json")"
  else
    failed=1
    log FAIL "mesh service $MESH_NAME is missing"
    print_snippet "$(cat "$TMP_DIR/mesh-service.err")" 20
  fi

  if kubectl -n "$NAMESPACE" get endpoints "$MESH_NAME" -o json >"$ep_json" 2>"$TMP_DIR/mesh-endpoints.err"; then
    local count
    count="$(jq '[.subsets[]?.addresses[]?] | length' "$ep_json")"
    log INFO "mesh endpoint address count: $count"
    jq -r '.subsets[]?.addresses[]?.ip' "$ep_json" | sed 's/^/    endpoint: /' || true
    if [[ "$count" -eq 0 ]]; then
      failed=1
      log FAIL "mesh service has no endpoints"
    else
      log PASS "mesh service has endpoints"
    fi
  else
    failed=1
    log FAIL "mesh endpoints $MESH_NAME are missing"
    print_snippet "$(cat "$TMP_DIR/mesh-endpoints.err")" 20
  fi

  if [[ "$failed" -eq 0 ]]; then
    STATUS_MESH_RESOURCE="pass"
    REASON_MESH_RESOURCE="mesh pod/service/endpoints look healthy"
  else
    STATUS_MESH_RESOURCE="fail"
    REASON_MESH_RESOURCE="mesh pod/service/endpoints are incomplete"
  fi
}

kubectl_exec_http() {
  local pod="$1"
  local desc="$2"
  local url="$3"
  local header="${4:-}"
  local output

  log RUN "kubectl exec HTTP check: $desc url=$url header=${header:-<none>}"
  output="$(
    timeout "$((HTTP_TIMEOUT + 5))s" kubectl -n "$NAMESPACE" exec "$pod" -- sh -c '
      url="$1"
      header="${2:-}"
      max_time="${3:-8}"
      if command -v wget >/dev/null 2>&1; then
        if [ -n "$header" ]; then
          wget -qO- -T "$max_time" --header="$header" "$url"
        else
          wget -qO- -T "$max_time" "$url"
        fi
      elif command -v curl >/dev/null 2>&1; then
        if [ -n "$header" ]; then
          curl -sS --max-time "$max_time" -H "$header" "$url"
        else
          curl -sS --max-time "$max_time" "$url"
        fi
      else
        echo "neither wget nor curl exists in pod"
        exit 127
      fi
    ' kt-doctor "$url" "$header" "$HTTP_TIMEOUT" 2>&1
  )"
  local code=$?
  print_snippet "$output" 40
  if [[ "$code" -eq 0 ]]; then
    log PASS "$desc replied"
  else
    log FAIL "$desc failed with exit $code"
  fi
  return "$code"
}

check_mesh_tunnel() {
  section "Mesh tunnel"
  if [[ "$SKIP_NETWORK_TESTS" -eq 1 ]]; then
    STATUS_MESH_TUNNEL="skip"
    REASON_MESH_TUNNEL="network tests skipped"
    log WARN "$REASON_MESH_TUNNEL"
    return 0
  fi

  if [[ "$STATUS_MESH_RESOURCE" != "pass" ]]; then
    STATUS_MESH_TUNNEL="skip"
    REASON_MESH_TUNNEL="mesh resource check did not pass"
    log WARN "$REASON_MESH_TUNNEL"
    return 0
  fi

  if kubectl_exec_http "$MESH_NAME" "mesh pod -> local app through reverse tunnel" "http://127.0.0.1:${REMOTE_PORT}${LOCAL_HEALTH_PATH}"; then
    STATUS_MESH_TUNNEL="pass"
    REASON_MESH_TUNNEL="mesh pod can reach local app"
  else
    STATUS_MESH_TUNNEL="fail"
    REASON_MESH_TUNNEL="mesh pod cannot reach local app"
  fi
}

check_ingress_path() {
  section "In-cluster header route"
  if [[ "$SKIP_NETWORK_TESTS" -eq 1 ]]; then
    STATUS_INGRESS="skip"
    REASON_INGRESS="network tests skipped"
    log WARN "$REASON_INGRESS"
    return 0
  fi

  if [[ "$STATUS_MESH_RESOURCE" != "pass" ]]; then
    STATUS_INGRESS="skip"
    REASON_INGRESS="mesh resource check did not pass"
    log WARN "$REASON_INGRESS"
    return 0
  fi

  local header="${VERSION_MARK%%:*}: ${VERSION_MARK#*:}"
  if kubectl_exec_http "$MESH_NAME" "mesh pod -> service with version header" "http://${SERVICE}:${REMOTE_PORT}${INGRESS_HEALTH_PATH}" "$header"; then
    STATUS_INGRESS="pass"
    REASON_INGRESS="service header route replied"
  else
    STATUS_INGRESS="fail"
    REASON_INGRESS="service header route did not reply"
  fi
}

http_probe() {
  local desc="$1"
  local url="$2"
  local output
  log RUN "curl $desc: $url"
  output="$(curl -sS -i --max-time "$HTTP_TIMEOUT" "$url" 2>&1)"
  local code=$?
  print_snippet "$output" 40
  if [[ "$code" -eq 0 ]]; then
    local status
    status="$(printf '%s\n' "$output" | awk '/^HTTP\// {code=$2} END {print code}')"
    log PASS "$desc replied with HTTP ${status:-unknown}"
  else
    log FAIL "$desc failed with curl exit $code"
  fi
  return "$code"
}

check_outbound_connect() {
  section "Outbound ktctl connect path"
  if [[ "$SKIP_NETWORK_TESTS" -eq 1 ]]; then
    STATUS_OUTBOUND="skip"
    REASON_OUTBOUND="network tests skipped"
    log WARN "$REASON_OUTBOUND"
    return 0
  fi

  if command -v getent >/dev/null 2>&1; then
    capture_cmd getent hosts "$OUTBOUND_SERVICE" || true
  else
    log WARN "getent not found; skip host resolution check"
  fi

  local ok=1
  if http_probe "$OUTBOUND_SERVICE business path" "http://${OUTBOUND_SERVICE}:${OUTBOUND_PORT}${OUTBOUND_PATH}"; then
    ok=0
  fi

  if http_probe "$OUTBOUND_SERVICE readiness path" "http://${OUTBOUND_SERVICE}:${OUTBOUND_PORT}${OUTBOUND_READINESS_PATH}"; then
    :
  else
    log WARN "readiness path failed; business path result is the primary outbound signal"
  fi

  if [[ "$ok" -eq 0 ]]; then
    STATUS_OUTBOUND="pass"
    REASON_OUTBOUND="outbound service replied quickly"
  else
    STATUS_OUTBOUND="fail"
    REASON_OUTBOUND="outbound service timed out or was unreachable"
  fi
}

check_routes_and_hosts() {
  section "Local routes and hosts"
  if command -v ip >/dev/null 2>&1; then
    capture_cmd ip link show kt0 || true
    capture_cmd ip route show dev kt0 || true
  else
    log WARN "ip command not found; skip route check"
  fi

  if [[ -r /etc/hosts ]]; then
    log INFO "/etc/hosts kt-related entries:"
    awk '
      /ktctl|svc.cluster|tea-shop|kt-connect|^[[:space:]]*10\./ {print "    " $0}
    ' /etc/hosts || true
  else
    log WARN "cannot read /etc/hosts"
  fi
}

check_pid_files() {
  section "Local kt pid files"
  local pid_dir="${HOME}/.kt/pid"
  local stale=0
  local total=0

  if [[ ! -d "$pid_dir" ]]; then
    STATUS_PID_FILES="pass"
    REASON_PID_FILES="pid directory does not exist"
    log PASS "$REASON_PID_FILES"
    return 0
  fi

  while IFS= read -r -d '' file; do
    total=$((total + 1))
    local pid
    pid="$(tr -cd '0-9' <"$file" 2>/dev/null || true)"
    if [[ -n "$pid" && "$pid" =~ ^[0-9]+$ && -d "/proc/$pid" ]]; then
      log PASS "$(basename "$file") -> pid $pid is alive"
    else
      stale=$((stale + 1))
      log WARN "$(basename "$file") is stale or unreadable"
      if [[ "$CLEAN_LOCAL_PIDS" -eq 1 ]]; then
        if rm -f "$file" 2>/dev/null; then
          log PASS "removed stale pid file $file"
        else
          log WARN "cannot remove stale pid file $file; permission may be root-owned"
        fi
      fi
    fi
  done < <(find "$pid_dir" -maxdepth 1 -type f -name '*.pid' -print0 2>/dev/null)

  if [[ "$total" -eq 0 || "$stale" -eq 0 ]]; then
    STATUS_PID_FILES="pass"
    REASON_PID_FILES="no stale pid files"
  else
    STATUS_PID_FILES="warn"
    REASON_PID_FILES="$stale stale pid file(s)"
  fi
}

recover_service_selector() {
  section "Repair target service selector"
  if [[ "$SERVICE_EXISTS" -ne 1 ]]; then
    log FAIL "cannot recover selector because service $SERVICE is not readable"
    return 1
  fi
  if [[ "$ORPHAN_ROUTER" -ne 1 && "$FORCE_RECOVER_SERVICE" -ne 1 ]]; then
    log WARN "service is not detected as orphaned; skip selector recovery"
    return 0
  fi

  local selector_file="$TMP_DIR/original-selector.json"
  local patch_file="$TMP_DIR/recover-service-patch.json"
  if [[ -n "$ORIGINAL_SELECTOR" ]]; then
    printf '%s' "$ORIGINAL_SELECTOR" >"$selector_file"
  else
    log WARN "kt-selector annotation is empty; infer original selector by dropping kt-role/kt-target"
    printf '%s' "$CURRENT_SELECTOR_JSON" | jq 'del(."kt-role", ."kt-target")' >"$selector_file"
  fi
  if ! jq -e 'type == "object" and length > 0' "$selector_file" >/dev/null 2>&1; then
    log FAIL "cannot determine a non-empty original selector"
    return 1
  fi

  jq -n --slurpfile selector "$selector_file" \
    '[
      {"op":"replace","path":"/spec/selector","value":$selector[0]},
      {"op":"remove","path":"/metadata/annotations/kt-selector"}
    ]' >"$patch_file"

  log INFO "patch payload:"
  print_snippet "$(cat "$patch_file")" 20
  if run_cmd kubectl -n "$NAMESPACE" patch svc "$SERVICE" --type=json -p "$(cat "$patch_file")"; then
    return 0
  fi

  log WARN "json patch with annotation removal failed; retry replacing selector only"
  jq -n --slurpfile selector "$selector_file" \
    '[{"op":"replace","path":"/spec/selector","value":$selector[0]}]' >"$patch_file"
  log INFO "fallback patch payload:"
  print_snippet "$(cat "$patch_file")" 20
  run_cmd kubectl -n "$NAMESPACE" patch svc "$SERVICE" --type=json -p "$(cat "$patch_file")"
}

restart_user_unit() {
  local unit="$1"
  section "Restart $unit"
  run_cmd systemctl --user restart "$unit" || return 1
  local deadline=$((SECONDS + WAIT_TIMEOUT))
  while [[ "$SECONDS" -lt "$deadline" ]]; do
    local active
    active="$(systemctl --user is-active "$unit" 2>/dev/null || true)"
    if [[ "$active" == "active" ]]; then
      log PASS "$unit is active after restart"
      capture_cmd journalctl --user -u "$unit" -n 25 --no-pager || true
      return 0
    fi
    log INFO "waiting for $unit to become active; current=$active"
    sleep 2
  done
  log FAIL "$unit did not become active within ${WAIT_TIMEOUT}s"
  capture_cmd journalctl --user -u "$unit" -n 60 --no-pager || true
  return 1
}

wait_for_mesh_resources() {
  section "Wait for mesh resources"
  local deadline=$((SECONDS + WAIT_TIMEOUT))
  while [[ "$SECONDS" -lt "$deadline" ]]; do
    local phase=""
    local endpoints="0"
    phase="$(kubectl -n "$NAMESPACE" get pod "$MESH_NAME" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
    endpoints="$(kubectl -n "$NAMESPACE" get endpoints "$MESH_NAME" -o json 2>/dev/null | jq '[.subsets[]?.addresses[]?] | length' 2>/dev/null || printf '0')"
    log INFO "mesh pod phase=${phase:-missing}; endpoints=$endpoints"
    if [[ "$phase" == "Running" && "$endpoints" -gt 0 ]]; then
      log PASS "mesh resources are ready"
      return 0
    fi
    sleep 2
  done
  log FAIL "mesh resources did not become ready within ${WAIT_TIMEOUT}s"
  return 1
}

apply_fixes() {
  if [[ "$FIX" -ne 1 ]]; then
    return 0
  fi

  section "Fix plan"
  local need_connect_restart="$RESTART_CONNECT"
  local need_mesh_restart="$RESTART_MESH"
  local need_recover="$RECOVER_SERVICE"

  if [[ "$ORPHAN_ROUTER" -eq 1 ]]; then
    need_recover=1
    log WARN "detected orphan router; target service selector recovery is needed"
  fi
  if [[ "$STATUS_CONNECT" == "fail" || "$STATUS_OUTBOUND" == "fail" ]]; then
    need_connect_restart=1
    log WARN "connect unit/outbound path failed; connect restart is needed"
  fi
  if [[ "$STATUS_MESH_UNIT" == "fail" ]]; then
    need_mesh_restart=1
    log WARN "mesh unit failed; mesh restart is needed"
  fi
  if [[ "$STATUS_MESH_RESOURCE" == "fail" ]]; then
    need_mesh_restart=1
    log WARN "mesh resources are incomplete; mesh restart is needed"
  fi
  if [[ "$STATUS_MESH_TUNNEL" == "fail" || "$STATUS_INGRESS" == "fail" ]]; then
    if [[ "$LOCAL_PORT_LISTENING" -eq 1 ]]; then
      need_mesh_restart=1
      log WARN "mesh tunnel/ingress failed while local port is listening; mesh restart is needed"
    else
      log WARN "mesh path failed but local app port is not listening; start the local app before restarting mesh"
    fi
  fi

  if [[ "$need_recover" -eq 1 ]]; then
    recover_service_selector || true
  else
    log INFO "no service selector recovery planned"
  fi

  if [[ "$CLEAN_LOCAL_PIDS" -eq 1 ]]; then
    log INFO "local stale pid cleanup was requested"
  fi

  if [[ "$need_connect_restart" -eq 1 ]]; then
    if restart_user_unit "$CONNECT_UNIT"; then
      CONNECT_RESTARTED=1
      need_mesh_restart=1
      log WARN "connect restarted; mesh will be restarted to rebuild reverse tunnel"
    fi
  else
    log INFO "no connect restart planned"
  fi

  if [[ "$need_mesh_restart" -eq 1 ]]; then
    if restart_user_unit "$MESH_UNIT"; then
      MESH_RESTARTED=1
      wait_for_mesh_resources || true
    fi
  else
    log INFO "no mesh restart planned"
  fi
}

run_diagnosis() {
  check_systemd
  check_local_port
  list_kt_resources
  check_target_service
  check_mesh_resources
  check_mesh_tunnel
  check_ingress_path
  check_outbound_connect
  check_routes_and_hosts
  check_pid_files
}

rerun_after_fix() {
  if [[ "$FIX" -ne 1 ]]; then
    return 0
  fi
  if [[ "$CONNECT_RESTARTED" -eq 0 && "$MESH_RESTARTED" -eq 0 && "$RECOVER_SERVICE" -eq 0 && "$ORPHAN_ROUTER" -eq 0 ]]; then
    return 0
  fi

  section "Post-fix verification"
  check_systemd
  check_target_service
  check_mesh_resources
  check_mesh_tunnel
  check_ingress_path
  check_outbound_connect
}

summary_item() {
  local name="$1"
  local status="$2"
  local reason="$3"
  local marker="$status"
  case "$status" in
    pass) marker="${C_GREEN}PASS${C_RESET}" ;;
    warn) marker="${C_YELLOW}WARN${C_RESET}" ;;
    fail) marker="${C_RED}FAIL${C_RESET}" ;;
    skip) marker="${C_YELLOW}SKIP${C_RESET}" ;;
  esac
  printf '  %-24s %-10s %s\n' "$name" "$marker" "$reason"
}

print_summary() {
  section "Summary"
  summary_item "connect unit" "$STATUS_CONNECT" "$REASON_CONNECT"
  summary_item "mesh unit" "$STATUS_MESH_UNIT" "$REASON_MESH_UNIT"
  summary_item "local app port" "$STATUS_LOCAL_PORT" "$REASON_LOCAL_PORT"
  summary_item "target service" "$STATUS_TARGET_SERVICE" "$REASON_TARGET_SERVICE"
  summary_item "router state" "$STATUS_ROUTER" "$REASON_ROUTER"
  summary_item "mesh resources" "$STATUS_MESH_RESOURCE" "$REASON_MESH_RESOURCE"
  summary_item "mesh tunnel" "$STATUS_MESH_TUNNEL" "$REASON_MESH_TUNNEL"
  summary_item "ingress path" "$STATUS_INGRESS" "$REASON_INGRESS"
  summary_item "outbound path" "$STATUS_OUTBOUND" "$REASON_OUTBOUND"
  summary_item "local pid files" "$STATUS_PID_FILES" "$REASON_PID_FILES"

  if [[ "$FIX" -ne 1 ]]; then
    log INFO "diagnose-only mode; rerun with --fix to apply safe local repairs"
  fi
  if [[ "$NO_LOG_FILE" -eq 0 ]]; then
    log INFO "full log saved at $LOG_FILE"
  fi
}

exit_code_from_summary() {
  local code=0
  local status
  for status in \
    "$STATUS_CONNECT" \
    "$STATUS_MESH_UNIT" \
    "$STATUS_LOCAL_PORT" \
    "$STATUS_TARGET_SERVICE" \
    "$STATUS_ROUTER" \
    "$STATUS_MESH_RESOURCE" \
    "$STATUS_MESH_TUNNEL" \
    "$STATUS_INGRESS" \
    "$STATUS_OUTBOUND"; do
    if [[ "$status" == "fail" ]]; then
      code=1
    fi
  done
  return "$code"
}

main() {
  init
  show_context
  run_diagnosis
  apply_fixes
  rerun_after_fix
  print_summary
  exit_code_from_summary
}

main "$@"
