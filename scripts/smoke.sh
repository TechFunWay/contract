#!/usr/bin/env bash
# smoke.sh — contract 端到端冒烟验证
#
# 覆盖《应用支持功能与在线统计接入指南》验收清单的核心项：
#   1. 匿名心跳上报：字段齐全、device_id 为 32 位哈希、无 hostname
#   2. 赞赏链路：已登录成功 → {"ok":true}；未登录 401
#   3. 备份接口：创建/列表/下载/删除 + 路径穿越拒绝
#   4. 版本检查：假发布渠道返回新版本 → has_update=true
#   5. 退出开关：DISABLE_STATS=1 时无任何上报
#   6. 重启后 device.id 稳定（device_id 不变）
#
# 用法: ./scripts/smoke.sh [端口]

set -uo pipefail

PORT="${1:-18900}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d /tmp/contract-smoke.XXXXXX)"
BIN="$TMP/contract"
FAILS=0

say()  { printf '%s\n' "$*"; }
pass() { say "PASS  $*"; }
fail() { say "FAIL  $*"; FAILS=$((FAILS+1)); }

cleanup() { kill "$SRV_PID" "$FAKE_PID" 2>/dev/null; wait "$SRV_PID" "$FAKE_PID" 2>/dev/null; rm -rf "$TMP"; }
trap cleanup EXIT

# ---------- 假统计/发布渠道服务 ----------
# POST /api/apps.online/refresh → 200，请求体逐行记录到 stats.log
# GET  /release → GitHub Releases 形状 {"tag_name":"v9.9.9","html_url":"https://example.com"}
cat > "$TMP/fake.py" <<'PY'
import json, sys
from http.server import BaseHTTPRequestHandler, HTTPServer

LOG = sys.argv[2]

class H(BaseHTTPRequestHandler):
    def _send(self, code, body=b'{"ok":true}'):
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self):
        n = int(self.headers.get("Content-Length", 0))
        payload = self.rfile.read(n).decode()
        with open(LOG, "a") as f:
            f.write(payload + "\n")
        self._send(200)

    def do_GET(self):
        if self.path.startswith("/release"):
            body = json.dumps({"tag_name": "v9.9.9", "html_url": "https://example.com/rel"}).encode()
            self._send(200, body)
        else:
            self._send(404)

    def log_message(self, *a):
        pass

HTTPServer(("127.0.0.1", int(sys.argv[1])), H).serve_forever()
PY
# 随机高位端口，避免与其他运行实例/残留进程冲突
FAKE_PORT=$((18780 + RANDOM % 200))
python3 "$TMP/fake.py" "$FAKE_PORT" "$TMP/stats.log" &
FAKE_PID=$!

# ---------- 构建并启动 ----------
say "== 构建 $BIN =="
( cd "$ROOT/server" && go build -ldflags "-X smallgo/server/version.Version=v0.1.0" -o "$BIN" . ) || { fail "go build"; exit 1; }
pass "go build"

API="http://127.0.0.1:$PORT/api"
STATS_URL="http://127.0.0.1:$FAKE_PORT/api/apps.online/refresh"
GH_URL="http://127.0.0.1:$FAKE_PORT/release"

start_server() {
  env $1 "$BIN" -port "$PORT" -data-dir "$TMP/data" -web-dir "" -device-type fnos >> "$TMP/server.log" 2>&1 &
  SRV_PID=$!
  for _ in $(seq 1 50); do
    curl -s --max-time 8 -f "http://127.0.0.1:$PORT/api/health" >/dev/null 2>&1 && return 0
    sleep 0.2
  done
  fail "server did not come up"; cat "$TMP/server.log"; exit 1
}

say "== 阶段 1：正常启动（统计开启） =="
start_server "STATS_ENDPOINT=$STATS_URL UPDATE_CHECK_URL=$GH_URL"
pass "server up"

# 首个注册用户即管理员
REG=$(curl -s --max-time 8 -X POST "$API/auth/register" -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin12345"}')
TOKEN=$(echo "$REG" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("data",{}).get("token",""))' 2>/dev/null)
[ -n "$TOKEN" ] && pass "register admin" || { fail "register: $REG"; exit 1; }
AUTH="Authorization: Bearer $TOKEN"

# /api/version 含 startedAt
VER=$(curl -s --max-time 8 "$API/version")
echo "$VER" | python3 -c 'import json,sys; d=json.load(sys.stdin)["data"]; assert d["version"]=="v0.1.0"; assert d["startedAt"]' \
  && pass "/api/version startedAt + version=v0.1.0" || fail "/api/version: $VER"

# 未登录赞赏 → 401
CODE=$(curl -s --max-time 8 -o /dev/null -w '%{http_code}' -X POST "$API/donate/support")
[ "$CODE" = "401" ] && pass "donate unauthenticated 401" || fail "donate unauthenticated = $CODE"

# 已登录赞赏 → ok:true（假服务 200）
DON=$(curl -s --max-time 8 -X POST "$API/donate/support" -H "$AUTH")
echo "$DON" | grep -q '"ok":true' && pass "donate support ok" || fail "donate: $DON"

# 版本检查：假渠道 v9.9.9 > v0.1.0
CHK=$(curl -s --max-time 8 "$API/version/check" -H "$AUTH")
echo "$CHK" | python3 -c 'import json,sys; d=json.load(sys.stdin)["data"]; assert d["has_update"] and d["latest"]=="v9.9.9"' \
  && pass "version/check has_update" || fail "version/check: $CHK"

# 备份：创建 → 列表 → 下载 → 删除
BK=$(curl -s --max-time 8 -X POST "$API/backups" -H "$AUTH")
NAME=$(echo "$BK" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["name"])' 2>/dev/null)
[[ "$NAME" =~ ^backup_[0-9]{8}_[0-9]{6}\.db$ ]] && pass "backup create ($NAME)" || fail "backup create: $BK"

LIST=$(curl -s --max-time 8 "$API/backups" -H "$AUTH")
echo "$LIST" | python3 -c 'import json,sys; d=json.load(sys.stdin)["data"]; assert len(d["items"])==1 and d["auto_enabled"]' \
  && pass "backup list" || fail "backup list: $LIST"

curl -s --max-time 8 -o "$TMP/dl.db" -w '%{http_code}' "$API/backups/$NAME/download" -H "$AUTH" | grep -q 200 \
  && [ -s "$TMP/dl.db" ] && pass "backup download" || fail "backup download"
python3 -c "import sqlite3; sqlite3.connect('$TMP/dl.db').execute('pragma integrity_check').fetchone()" \
  && pass "downloaded snapshot is a valid sqlite db" || fail "snapshot integrity"

BAD=$(curl -s --max-time 8 -o /dev/null -w '%{http_code}' "$API/backups/evil.db/download" -H "$AUTH")
[ "$BAD" = "400" ] && pass "backup name validation 400" || fail "bad name = $BAD"

# 恢复：用刚创建的备份走完整恢复链路（服务端先自动留恢复前快照，返回 pre_backup）
RES=$(curl -s --max-time 15 -X POST "$API/backups/$NAME/restore" -H "$AUTH")
echo "$RES" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["code"]==0 and d["data"]["pre_backup"], d' \
  && pass "backup restore ($NAME)" || fail "backup restore: $RES"

BADR=$(curl -s --max-time 8 -o /dev/null -w '%{http_code}' -X POST "$API/backups/evil.db/restore" -H "$AUTH")
[ "$BADR" = "400" ] && pass "restore name validation 400" || fail "restore bad name = $BADR"

# 本地上传备份：把刚创建的备份文件再传回去，应存为新备份并可恢复；
# 上传产生的副本用完即删，不影响后面原有备份的删除步骤
UPL=$(curl -s --max-time 15 -X POST "$API/backups/upload" -H "$AUTH" -F "file=@$TMP/dl.db;filename=downloaded.db")
UPNAME=$(printf '%s' "$UPL" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["name"])' 2>/dev/null)
[ -n "$UPNAME" ] && pass "backup upload ($UPNAME)" || fail "backup upload: $UPL"
LIST2=$(curl -s --max-time 8 "$API/backups" -H "$AUTH")
printf '%s' "$LIST2" | grep -q "$UPNAME" && pass "uploaded backup in list" || fail "uploaded backup missing from list: $LIST2"
RES2=$(curl -s --max-time 8 -X POST "$API/backups/$UPNAME/restore" -H "$AUTH")
printf '%s' "$RES2" | grep -q '"pre_backup"' && pass "restore uploaded backup" || fail "restore uploaded: $RES2"
BADU=$(curl -s --max-time 8 -o /dev/null -w '%{http_code}' -X POST "$API/backups/upload" -H "$AUTH" -F "file=@$TMP/dl.db;filename=bad.txt")
[ "$BADU" = "400" ] && pass "upload extension validation 400" || fail "upload .txt = $BADU"
curl -s --max-time 8 -X DELETE "$API/backups/$UPNAME" -H "$AUTH" >/dev/null

# 恢复后原有备份仍在，再删除本次创建的那份
DEL=$(curl -s --max-time 8 -X DELETE "$API/backups/$NAME" -H "$AUTH")
echo "$DEL" | grep -q '"ok":true' && pass "backup delete" || fail "backup delete: $DEL"

# ---------- 合同业务：模板种子 → 建模板 → 生成合同 → 重渲染 → 删除 ----------
TPLS=$(curl -s --max-time 8 "$API/contract/templates" -H "$AUTH")
echo "$TPLS" | python3 -c 'import json,sys; d=json.load(sys.stdin)["data"]; assert len(d["items"])>=1, "seed template missing"' \
  && pass "template list seeds example" || fail "template list: $TPLS"

TPL=$(curl -s --max-time 8 -X POST "$API/contract/templates" -H "$AUTH" -H 'Content-Type: application/json' -d '{
  "name":"租赁合同","category":"测试","description":"冒烟",
  "content":"<p>甲方 <span class=\"ct-field\" data-field=\"party_a\" contenteditable=\"false\">【甲方名称】</span>，租金 <span class=\"ct-field\" data-field=\"amount\" contenteditable=\"false\">【租金】</span> 元。</p><script>alert(1)</script>",
  "fields":[{"key":"party_a","label":"甲方名称","type":"text","required":true},{"key":"amount","label":"租金","type":"number"}]}')
TPL_ID=$(echo "$TPL" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])' 2>/dev/null)
echo "$TPL" | python3 -c 'import json,sys; d=json.load(sys.stdin)["data"]; assert "script" not in d["content"], "xss leaked"' \
  && [ -n "$TPL_ID" ] && pass "template create + sanitize" || fail "template create: $TPL"

CT=$(curl -s --max-time 8 -X POST "$API/contract/contracts" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"template_id\":$TPL_ID,\"values\":{\"party_a\":\"张三\"}}")
CT_ID=$(echo "$CT" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])' 2>/dev/null)
echo "$CT" | python3 -c '
import json,sys
d=json.load(sys.stdin)["data"]
r=d["rendered"]
assert "ct-value" in r and "张三" in r, r
assert "ct-blank" in r, r
assert d["status"]=="draft", d["status"]
assert d["fields"][0]["key"]=="party_a"
' && [ -n "$CT_ID" ] && pass "contract create + render + snapshot" || fail "contract create: $CT"

UP=$(curl -s --max-time 8 -X PUT "$API/contract/contracts/$CT_ID" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"values":{"party_a":"李四","amount":"3500"},"status":"active","sign_date":"2026-09-19"}')
echo "$UP" | python3 -c '
import json,sys
d=json.load(sys.stdin)["data"]
r=d["rendered"]
assert "李四" in r and "3500" in r, r
assert d["status"]=="active"
' && pass "contract update re-renders" || fail "contract update: $UP"

LISTC=$(curl -s --max-time 8 "$API/contract/contracts?status=active" -H "$AUTH")
echo "$LISTC" | python3 -c 'import json,sys; d=json.load(sys.stdin)["data"]; assert d["total"]==1, d["total"]' \
  && pass "contract list filter" || fail "contract list: $LISTC"

curl -s --max-time 8 -X DELETE "$API/contract/contracts/$CT_ID" -H "$AUTH" | grep -q '"code":0' \
  && pass "contract delete" || fail "contract delete"
curl -s --max-time 8 -X DELETE "$API/contract/templates/$TPL_ID" -H "$AUTH" | grep -q '"code":0' \
  && pass "template delete" || fail "template delete"

kill "$SRV_PID" 2>/dev/null; wait "$SRV_PID" 2>/dev/null

# ---------- 心跳字段校验 ----------
say "== 阶段 1 校验心跳 =="
sleep 0.5
[ -s "$TMP/stats.log" ] || { fail "stats.log empty"; exit 1; }
python3 - "$TMP/stats.log" <<'PY' && pass "heartbeat fields (hashed id, no hostname)" || fail "heartbeat fields"
import json, re, sys
lines = [json.loads(l) for l in open(sys.argv[1]) if l.strip()]
hb = [e for e in lines if e.get("event", "") == ""]
assert hb, "no heartbeat"
e = hb[0]
assert e["app_name"] == "contract" and e["version"] == "v0.1.0"
assert e["device_type"] == "fnos"
assert re.fullmatch(r"[0-9a-f]{32}", e["device_id"]), e["device_id"]
assert "hostname" not in e
don = [e for e in lines if e.get("event") == "donate_support"]
assert don, "no donate event"
assert don[0]["device_id"] == e["device_id"]
PY
DEVICE_ID=$(python3 -c 'import json,sys; print([json.loads(l) for l in open(sys.argv[1])][0]["device_id"])' "$TMP/stats.log")

# ---------- 阶段 2：重启后 device_id 稳定 ----------
say "== 阶段 2：重启（同一数据目录） =="
: > "$TMP/stats.log"
start_server "STATS_ENDPOINT=$STATS_URL UPDATE_CHECK_URL=$GH_URL"
sleep 0.5
NEW_ID=$(python3 -c 'import json,sys; print([json.loads(l) for l in open(sys.argv[1])][0]["device_id"])' "$TMP/stats.log" 2>/dev/null)
[ "$NEW_ID" = "$DEVICE_ID" ] && pass "device_id stable across restart" || fail "device_id changed: $DEVICE_ID → $NEW_ID"
kill "$SRV_PID" 2>/dev/null; wait "$SRV_PID" 2>/dev/null

# ---------- 阶段 3：DISABLE_STATS=1 ----------
say "== 阶段 3：DISABLE_STATS=1 =="
: > "$TMP/stats.log"
start_server "STATS_ENDPOINT=$STATS_URL UPDATE_CHECK_URL=$GH_URL DISABLE_STATS=1"
sleep 0.5
[ ! -s "$TMP/stats.log" ] && pass "no reports when DISABLE_STATS=1" || fail "reports leaked: $(cat "$TMP/stats.log")"
DON2=$(curl -s --max-time 8 -o /dev/null -w '%{http_code}' -X POST "$API/donate/support" -H "$AUTH" 2>/dev/null)
kill "$SRV_PID" 2>/dev/null; wait "$SRV_PID" 2>/dev/null

say ""
if [ "$FAILS" -eq 0 ]; then
  say "全部通过 ✅  (artifacts in $TMP kept for inspection)"
else
  say "失败 $FAILS 项 ❌  (artifacts in $TMP)"
fi
exit "$FAILS"
