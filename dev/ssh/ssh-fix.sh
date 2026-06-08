#!/bin/sh
# One-time repair, run over an open SSH session. Copy to the device and run once.
#
# Two jobs:
#  1. Rewrite /usr/local/dropbear/on-boot.sh to a known-good version (the upstream
#     kobopatch-ssh starter shipped with a bad -E flag → dropbear never started).
#  2. Fix /.ssh ownership/perms so dropbear's strict pubkey check accepts the key.
#
# ⚠️ This fixes /.ssh but NOT the home directory itself. The LOAD-BEARING fix is
#    `chown 0:0 /` (root's home is / on this device; dropbear rejects every key if
#    the home dir is not owned by root). Do that separately — see dev/README.md.
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
