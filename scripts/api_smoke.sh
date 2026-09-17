#!/usr/bin/env bash
set -euo pipefail

api_root="${API_ROOT:-http://127.0.0.1:19533/api/v1}"
health_root="${HEALTH_ROOT:-http://127.0.0.1:19533}"
body_file="$(mktemp)"
trap 'rm -f "$body_file"' EXIT
last_body=""
checks=0

request() {
  local label="$1" expected="$2" method="$3" path="$4"
  local token="${5:-}" payload="${6:-}" idempotency="${7:-}"
  local args=(-sS -o "$body_file" -w "%{http_code}" -X "$method")
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  [[ -n "$payload" ]] && args+=(-H "Content-Type: application/json" --data "$payload")
  [[ -n "$idempotency" ]] && args+=(-H "Idempotency-Key: $idempotency")
  local status
  status="$(curl "${args[@]}" "$api_root$path")"
  last_body="$(cat "$body_file")"
  checks=$((checks + 1))
  if [[ "$status" != "$expected" ]]; then
    printf 'FAIL %-42s expected=%s actual=%s body=%s\n' "$label" "$expected" "$status" "$last_body" >&2
    exit 1
  fi
  printf 'PASS %-42s HTTP %s\n' "$label" "$status"
}

require_json() {
  local expression="$1" message="$2"
  if ! jq -e "$expression" >/dev/null <<<"$last_body"; then
    printf 'FAIL response assertion: %s body=%s\n' "$message" "$last_body" >&2
    exit 1
  fi
}

status="$(curl -sS -o "$body_file" -w "%{http_code}" "$health_root/healthz")"
[[ "$status" == "200" ]] || { printf 'FAIL health expected=200 actual=%s\n' "$status" >&2; exit 1; }
checks=$((checks + 1)); printf 'PASS %-42s HTTP 200\n' "backend health"

request "unauthenticated cells denied" 401 GET "/cells"
request "engineer login" 200 POST "/auth/login" "" '{"username":"engineer","password":"Safety#533"}'
engineer_token="$(jq -r '.data.token' <<<"$last_body")"
request "programmer login" 200 POST "/auth/login" "" '{"username":"programmer","password":"Program#533"}'
programmer_token="$(jq -r '.data.token' <<<"$last_body")"
request "reviewer login" 200 POST "/auth/login" "" '{"username":"reviewer","password":"Review#533"}'
reviewer_token="$(jq -r '.data.token' <<<"$last_body")"
request "auditor login" 200 POST "/auth/login" "" '{"username":"auditor","password":"Audit#533"}'
auditor_token="$(jq -r '.data.token' <<<"$last_body")"
request "admin login" 200 POST "/auth/login" "" '{"username":"admin","password":"Admin#533"}'
admin_token="$(jq -r '.data.token' <<<"$last_body")"

request "list seeded cells" 200 GET "/cells?page_size=100" "$engineer_token"
require_json '(.data | length) >= 1' "seeded cell list"
request "list seeded zones" 200 GET "/zones?page_size=100" "$engineer_token"
require_json '(.data | length) >= 3' "seeded zone list"
request "list seeded programs" 200 GET "/programs?page_size=100" "$engineer_token"
request "list seeded validation runs" 200 GET "/validations?page_size=100" "$engineer_token"

cell_payload='{"cell_code":"QA-CELL-533","name":"QA envelope cell","layout_geojson":{"type":"FeatureCollection","features":[]},"robot_model":"QA-Robot-X","controller_model":"QA-Control-X","max_reach_mm":2400,"owner_team":"QA Integration"}'
request "programmer cannot create cell" 403 POST "/cells" "$programmer_token" "$cell_payload"
request "engineer creates robot cell" 201 POST "/cells" "$engineer_token" "$cell_payload"
cell_id="$(jq -r '.data.id' <<<"$last_body")"
require_json '.data.cell_state == "draft" and .data.layout_version == 1' "new cell state and version"
request "robot cell detail" 200 GET "/cells/$cell_id" "$auditor_token"

invalid_zone="$(jq -nc --argjson cell "$cell_id" '{robot_cell_id:$cell,name:"Invalid bow tie",zone_type:"restricted",polygon_geojson:{type:"Polygon",coordinates:[[[0,0],[500,500],[0,500],[500,0],[0,0]]]},min_height_mm:0,max_height_mm:2000,speed_limit_mm_s:100,access_rule:"Must reject self intersection"}')"
request "self-intersecting zone rejected" 422 POST "/zones" "$engineer_token" "$invalid_zone"
require_json '.error.code == "invalid_geometry"' "invalid geometry error code"
zone_payload="$(jq -nc --argjson cell "$cell_id" '{robot_cell_id:$cell,name:"QA restricted gate",zone_type:"restricted",polygon_geojson:{type:"Polygon",coordinates:[[[800,-400],[1500,-400],[1500,400],[800,400],[800,-400]]]},min_height_mm:0,max_height_mm:2200,speed_limit_mm_s:100,access_rule:"Gate lock must precede motion"}')"
request "engineer creates safety zone" 201 POST "/zones" "$engineer_token" "$zone_payload"
zone_id="$(jq -r '.data.id' <<<"$last_body")"; zone_version="$(jq -r '.data.version' <<<"$last_body")"
request "activate safety zone" 200 POST "/zones/$zone_id/activate" "$engineer_token" "$(jq -nc --argjson version "$zone_version" '{version:$version}')"
require_json '.data.zone_state == "active" and .data.version == 2' "zone activation increments version"
request "stale zone activation rejected" 409 POST "/zones/$zone_id/activate" "$engineer_token" "$(jq -nc --argjson version "$zone_version" '{version:$version}')"

program_payload="$(jq -nc --argjson cell "$cell_id" '{robot_cell_id:$cell,program_code:"QA-MOVE-533",version:1,trajectory:[{x_mm:0,y_mm:0,z_mm:700,time_ms:0,speed_mm_s:450},{x_mm:1100,y_mm:0,z_mm:800,time_ms:2500,speed_mm_s:450},{x_mm:1600,y_mm:200,z_mm:850,time_ms:4000,speed_mm_s:350}],tool_radius_mm:160,payload_radius_mm:100,interlock_sequence:[{name:"emergency_stop_reset",sequence:1,depends_on:[]},{name:"gate_locked",sequence:2,depends_on:["emergency_stop_reset"]},{name:"light_curtain_clear",sequence:3,depends_on:["gate_locked"]}]}' )"
request "engineer cannot import program" 403 POST "/programs" "$engineer_token" "$program_payload"
request "programmer imports program" 201 POST "/programs" "$programmer_token" "$program_payload"
program_id="$(jq -r '.data.id' <<<"$last_body")"
require_json '.data.program_state == "uploaded" and (.data.source_checksum | length) == 64' "uploaded state and checksum"
request "motion program detail" 200 GET "/programs/$program_id" "$auditor_token"
request "illegal uploaded to active rejected" 409 POST "/programs/$program_id/transition" "$programmer_token" '{"target_state":"active"}'
request "parse program" 200 POST "/programs/$program_id/transition" "$programmer_token" '{"target_state":"parsed"}'
request "mark program ready" 200 POST "/programs/$program_id/transition" "$programmer_token" '{"target_state":"ready"}'
request "activate program" 200 POST "/programs/$program_id/transition" "$programmer_token" '{"target_state":"active"}'
require_json '.data.program_state == "active"' "program active"

idempotency="qa-validation-533-main"
request "run envelope simulation" 201 POST "/validations" "$engineer_token" "$(jq -nc --argjson program "$program_id" '{motion_program_id:$program,retry_failed:false}')" "$idempotency"
run_id="$(jq -r '.data.id' <<<"$last_body")"
require_json '.data.validation_status == "failed" and (.data.collision_events | length) >= 1 and .data.collision_events[0].evidence != ""' "collision evidence and failed status"
request "idempotency key reuses run" 200 POST "/validations" "$engineer_token" "$(jq -nc --argjson program "$program_id" '{motion_program_id:$program,retry_failed:false}')" "$idempotency"
require_json ".data.id == $run_id and .data.reused == true" "same idempotency response"
request "validation detail" 200 GET "/validations/$run_id" "$auditor_token"
request "programmer cannot review" 403 POST "/validations/$run_id/review" "$programmer_token" '{"note":"Program uploader must not review."}'
request "reviewer records review" 200 POST "/validations/$run_id/review" "$reviewer_token" '{"note":"Independent offline evidence review completed."}'
require_json '.data.validation_status == "reviewed"' "reviewed state"
request "reviewer accepts evidence" 200 POST "/validations/$run_id/accept" "$reviewer_token" '{"note":"Evidence accepted for planning; site authority remains separate."}'
require_json '.data.validation_status == "accepted" and .data.risk_score > 0' "acceptance preserves objective risk"

self_program="$(jq -nc --argjson cell "$cell_id" '{robot_cell_id:$cell,program_code:"QA-SELF-533",version:1,trajectory:[{x_mm:0,y_mm:0,z_mm:700,time_ms:0},{x_mm:1200,y_mm:0,z_mm:700,time_ms:3000}],tool_radius_mm:100,payload_radius_mm:80,interlock_sequence:[{name:"gate_locked",sequence:1,depends_on:[]}]}' )"
request "admin imports self-review program" 201 POST "/programs" "$admin_token" "$self_program"
self_program_id="$(jq -r '.data.id' <<<"$last_body")"
request "admin parses own program" 200 POST "/programs/$self_program_id/transition" "$admin_token" '{"target_state":"parsed"}'
request "admin readies own program" 200 POST "/programs/$self_program_id/transition" "$admin_token" '{"target_state":"ready"}'
request "engineer validates admin program" 201 POST "/validations" "$engineer_token" "$(jq -nc --argjson program "$self_program_id" '{motion_program_id:$program,retry_failed:false}')" "qa-validation-533-self"
self_run_id="$(jq -r '.data.id' <<<"$last_body")"
request "admin reviews own uploaded program" 200 POST "/validations/$self_run_id/review" "$admin_token" '{"note":"Review step recorded before self-acceptance guard."}'
request "uploader self-acceptance denied" 403 POST "/validations/$self_run_id/accept" "$admin_token" '{"note":"This acceptance must be denied by uploader isolation."}'
require_json '.error.code == "forbidden"' "self acceptance error"

request "auditor reads audit stream" 200 GET "/audit?page_size=150" "$auditor_token"
require_json '([.data[].resource_type] | unique | length) == 4 and ([.data[].action] | index("validation_run.accepted")) != null' "all four entity projections and accepted action"

printf 'ALL %d API CHECKS PASSED\n' "$checks"
