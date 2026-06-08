#!/bin/sh
# One-time repair, run over an open SSH session. Copy to the device and run once.
#
# Three jobs:
#  1. Rewrite /usr/local/dropbear/on-boot.sh to a known-good version (the upstream
#     kobopatch-ssh starter shipped with a bad -E flag → dropbear never started).
#  2. `chown 0:0 /` — the LOAD-BEARING fix. Root's home is / on this device, and
#     `/` shipped owned by uid 501; dropbear's strict pubkey check rejects every
#     key when the home dir is not owned by root, so key-only auth silently failed
#     until this. Non-recursive (just the / inode), idempotent, reversible with
#     `chown 501:root /`. Persists on /dev/root (ext4). See dev/README.md for why.
#  3. Fix /.ssh ownership/perms so the pubkey file passes the same strict check.
DBDIR=/etc/dropbear
KEY="$DBDIR/dropbear_ed25519_host_key"
LOG=/usr/local/dropbear/dropbear.log
mkdir -p "$DBDIR"
# rewrite on-boot.sh without the bad -E flag (keep in sync with dev/ssh/on-boot.sh)
cat > /usr/local/dropbear/on-boot.sh <<'INNER'
#!/bin/sh
DBDIR=/etc/dropbear
KEY="$DBDIR/dropbear_ed25519_host_key"
LOG=/usr/local/dropbear/dropbear.log
mkdir -p "$DBDIR"
[ -f "$KEY" ] || /usr/local/bin/dropbearkey -t ed25519 -f "$KEY" >>"$LOG" 2>&1
case "$(pidof dropbear | wc -w)" in
0) /usr/local/bin/dropbear -s -g -p 22 -r "$KEY" >>"$LOG" 2>&1 & ;;
esac
exit 0
INNER
chmod 755 /usr/local/dropbear/on-boot.sh
# load-bearing: root's home is /, which shipped owned by uid 501 → dropbear
# rejected every pubkey. Non-recursive + idempotent. Reverse: chown 501:root /
chown 0:0 /
# fix key ownership/perms
chown -R root:root /.ssh 2>/dev/null
chmod 700 /.ssh
chmod 600 /.ssh/authorized_keys
# generate host key if absent, then start
[ -f "$KEY" ] || /usr/local/bin/dropbearkey -t ed25519 -f "$KEY" >>"$LOG" 2>&1
case "$(pidof dropbear | wc -w)" in
0) /usr/local/bin/dropbear -s -g -p 22 -r "$KEY" >>"$LOG" 2>&1 & ;;
esac
sleep 2
echo "pid: $(pidof dropbear)"
echo "--- IP ---"
ifconfig 2>/dev/null | grep -w inet | grep -v 127.0.0.1
