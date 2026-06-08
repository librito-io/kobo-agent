#!/bin/sh
# Blank-root-password fallback. Copy to /mnt/onboard/.adds/librito/ssh-open.sh
# (user partition — survives a firmware update). Bound to the NickelMenu
# "SSH open" item so you can recover SSH with a device tap when key auth is
# broken (e.g. before the `chown 0:0 /` fix, or on a fresh device).
#
# Blanks root's password in /etc/shadow and restarts dropbear with -B (blank /
# "none" auth allowed) and password auth ON (no -s). This is the recovery hatch,
# NOT the steady state — key auth via on-boot.sh is. The blank-password path does
# NOT survive a reboot (rootfs /etc/shadow is reset on boot from the image).
#
# 🛑 Dev-only. The blank-password path MUST be stripped before any production /
#    installer ship (build plan Step 5).
KEY=/etc/dropbear/dropbear_ed25519_host_key
LOG=/usr/local/dropbear/dropbear.log
echo "=== root passwd/shadow before ==="
grep '^root:' /etc/passwd
grep '^root:' /etc/shadow 2>/dev/null
# blank root password: set the password field to empty
if [ -f /etc/shadow ]; then
  sed -i 's#^root:[^:]*:#root::#' /etc/shadow
  echo "shadow blanked"
else
  sed -i 's#^root:[^:]*:#root::#' /etc/passwd
  echo "passwd blanked"
fi
grep '^root:' /etc/shadow 2>/dev/null
# restart dropbear allowing blank-password root logins (-B), password auth on (no -s)
kill $(pidof dropbear) 2>/dev/null; sleep 1
/usr/local/bin/dropbear -B -p 22 -r "$KEY" >>"$LOG" 2>&1 &
sleep 1
echo "pid: $(pidof dropbear)"
ifconfig 2>/dev/null | grep -w inet | grep -v 127.0.0.1
