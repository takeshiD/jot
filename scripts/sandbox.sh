#!/bin/sh
# Run a command (default: ./bin/jot) against an ISOLATED Taskwarrior dataset.
# Never touches ~/.taskrc or ~/.task: TASKRC/TASKDATA point into $JOT_SANDBOX
# (default: $HOME/jot-sandbox, so with an isolated HOME everything stays there).
#
#   scripts/sandbox.sh [--seed] [--] [command args...]
#
# --seed   create sample tasks (only when the dataset is empty)
set -eu
here=$(cd "$(dirname "$0")/.." && pwd)
box=${JOT_SANDBOX:-$HOME/jot-sandbox}
mkdir -p "$box/data"
[ -f "$box/taskrc" ] || : > "$box/taskrc"
export TASKRC="$box/taskrc" TASKDATA="$box/data"
seed=0
while [ $# -gt 0 ]; do
  case "$1" in
    --seed) seed=1; shift ;;
    --) shift; break ;;
    *) break ;;
  esac
done
if [ "$seed" = 1 ] && [ ! -f "$box/seeded" ]; then
  q="rc.confirmation=off rc.verbose=nothing"
  task $q add project:work -- "週報を書く" >/dev/null
  task $q add project:work +mtg -- "定例ミーティングの議事録" >/dev/null
  task $q add project:home -- "牛乳を買う" >/dev/null
  task $q add -- "Read the Bubble Tea docs" >/dev/null
  task $q add project:work -- "とても長い日本語のタイトルのタスクで、狭い端末でも一行に収まるか確認するためのもの" >/dev/null
  : > "$box/seeded"
fi
if [ $# -eq 0 ]; then
  set -- "$here/bin/jot"
fi
exec "$@"
