#!/bin/sh
#
# collect-libs.sh <binary> <staging-root>
#
# Copy a dynamically linked binary and every shared object it needs into a
# staging tree that mirrors absolute paths, so the tree can be COPYed wholesale
# into a distroless final stage.
#
# Resolving with ldd rather than a hand-written list matters: the MySQL client
# packages bundle private copies of OpenSSL under /usr/lib/mysql/private, which
# a hardcoded list of /lib/x86_64-linux-gnu/* paths would silently miss. The
# result is an image that builds fine and then fails to start.
set -eu

binary="$1"
root="$2"

[ -x "$binary" ] || { echo "collect-libs: $binary is not executable" >&2; exit 1; }

install -D "$binary" "$root$binary"

# ldd prints either "name => /abs/path (0x...)" or a bare "/abs/path (0x...)"
# for the dynamic loader itself. Both are needed; take every absolute path.
#
# Split on tabs as well as spaces. ldd indents the loader line with a TAB, so
# splitting on spaces alone leaves the token as "\t/lib/ld-linux-...", which
# never matches ^/ and silently drops the one library nothing can run without.
# A base image that happens to ship its own loader hides the omission until you
# move to a base that does not, and the failure then reads
# "exec: no such file or directory" on a binary that is plainly present.
ldd "$binary" \
  | tr -s ' \t' '\n\n' \
  | grep '^/' \
  | sort -u \
  | while read -r lib; do
      [ -e "$lib" ] || continue
      install -D "$lib" "$root$lib"
    done

echo "collect-libs: staged $binary and $(find "$root" -name '*.so*' | wc -l) libraries"
