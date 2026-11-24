#!/bin/sh
# End-to-end helper for manual/automated acceptance (isolated data only).
# 1) seeds sample tasks with the plain `task` CLI (first run only)
# 2) runs Jot
# 3) after Jot exits, shows what plain `task` sees, then changes ANOTHER
#    task with the CLI (to prove Jot's Today order survives CLI edits)
# 4) waits for Enter
set -eu
here=$(cd "$(dirname "$0")/.." && pwd)
box=${JOT_SANDBOX:-$HOME/jot-sandbox}
JOT_SANDBOX=$box sh "$here/scripts/sandbox.sh" --seed -- "$here/bin/jot" "$@"
export TASKRC="$box/taskrc" TASKDATA="$box/data"
q="rc.confirmation=off rc.verbose=nothing rc.color=off"
r="rc.uda.jottoday.type=string rc.uda.jotorder.type=numeric rc.report.jt.columns=jotorder,description,status,tags,jottoday rc.report.jt.labels=ORD,DESC,STATUS,TAGS,TODAY rc.report.jt.sort=jotorder+ rc.report.jt.filter=jottoday.any:"
echo "=== task (CLI) sees Today items ==="
task $q $r jt 2>/dev/null || true
task $q /週報/ modify +clichanged >/dev/null 2>&1 || true
echo "=== CLI changed: 週報を書く +clichanged ==="
echo "press Enter"
read _ || true
