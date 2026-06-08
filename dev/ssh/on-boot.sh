#!/bin/sh
# Key-only dropbear starter. Install to /usr/local/dropbear/on-boot.sh (rootfs).
#
# Generates a host key on first run, then starts dropbear KEY-ONLY (-s disables
# password auth, -g disables root password login) on port 22. The `case` guard
# makes it idempotent — a second invocation while dropbear is already up is a
# no-op, so re-firing the launcher never spawns a duplicate.
#
# NOTE: this dropbear build has NO -E flag (the upstream kobopatch-ssh on-boot.sh
# uses -E and fails "Invalid option -E" → never starts; strip it).
DBDIR=/etc/dropbear
KEY="$DBDIR/dropbear_ed25519_host_key"
LOG=/usr/local/dropbear/dropbear.log
mkdir -p "$DBDIR"
[ -f "$KEY" ] || /usr/local/bin/dropbearkey -t ed25519 -f "$KEY" >>"$LOG" 2>&1
case "$(pidof dropbear | wc -w)" in
0) /usr/local/bin/dropbear -s -g -p 22 -r "$KEY" >>"$LOG" 2>&1 & ;;
esac
exit 0
