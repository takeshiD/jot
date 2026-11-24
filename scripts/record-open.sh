#!/bin/sh
# Stand-in "browser" for tests: records each argv element on its own line
# instead of opening anything. Log file: $JOT_OPEN_LOG (default $HOME/opened.log)
log=${JOT_OPEN_LOG:-$HOME/opened.log}
for a in "$@"; do printf 'ARG:%s\n' "$a" >> "$log"; done
printf 'END\n' >> "$log"
