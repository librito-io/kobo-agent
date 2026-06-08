#!/bin/sh
# Plant the dev pubkey so the key-only boot dropbear authenticates you.
# Copy to the device and run once over an open SSH session (or via "SSH open").
#
# Pubkey comes from $LIBRITO_DEV_PUBKEY — NEVER hardcode a personal key in this
# public repo. Export it before running, e.g.:
#   LIBRITO_DEV_PUBKEY="ssh-ed25519 AAAA... you@host" sh ssh-key.sh
#
# Plants into every candidate root home because dropbear reads
# $HOME/.ssh/authorized_keys and the root home has moved between firmwares.
# See dev/README.md for the load-bearing `chown 0:0 /` fix that makes the
# key-only check actually pass (root's home is / on this device).
if [ -z "$LIBRITO_DEV_PUBKEY" ]; then
  echo "ERROR: set LIBRITO_DEV_PUBKEY to your ssh public key first" >&2
  exit 1
fi
echo "=== root passwd entry ==="
grep '^root:' /etc/passwd
ROOTHOME=$(grep '^root:' /etc/passwd | cut -d: -f6)
echo "root home = [$ROOTHOME]"
for H in / "$ROOTHOME" /root; do
  [ -n "$H" ] || continue
  mkdir -p "$H/.ssh"
  echo "$LIBRITO_DEV_PUBKEY" > "$H/.ssh/authorized_keys"
  chown -R 0:0 "$H/.ssh"
  chmod 700 "$H/.ssh"; chmod 600 "$H/.ssh/authorized_keys"
  echo "planted: $H/.ssh  ->  $(ls -la "$H/.ssh/authorized_keys")"
done
kill $(pidof dropbear) 2>/dev/null; sleep 1
/usr/local/bin/dropbear -s -g -p 22 -r /etc/dropbear/dropbear_ed25519_host_key >>/usr/local/dropbear/dropbear.log 2>&1 &
sleep 1
echo "dropbear pid: $(pidof dropbear)"
