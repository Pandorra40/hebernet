#!/usr/bin/env bash
# Active usrquota sur /home (ou /) — best effort. Safe to re-run.
set -euo pipefail

[[ "$(id -u)" -eq 0 ]] || { echo "lancez en root : sudo $0"; exit 1; }

log() { printf '==> %s\n' "$*"; }

# Cible : FS de /home si dédié, sinon /
MNT="$(findmnt -n -o TARGET --target /home 2>/dev/null || true)"
[[ -n "$MNT" ]] || MNT=/
FSTYPE="$(findmnt -n -o FSTYPE "$MNT" 2>/dev/null || echo unknown)"
OPTS="$(findmnt -n -o OPTIONS "$MNT" 2>/dev/null || true)"

log "Montage $MNT (fstype=$FSTYPE)"

if [[ "$FSTYPE" == "xfs" ]]; then
  log "XFS : les quotas projet/user se gèrent via xfs_quota (non automatisé ici)"
  exit 0
fi

if echo "$OPTS" | grep -qw usrquota; then
  log "usrquota déjà actif"
else
  log "Remontage avec usrquota…"
  if ! mount -o remount,usrquota "$MNT" 2>/dev/null; then
    # Persistance fstab
    if [[ -f /etc/fstab ]] && ! grep -E "[[:space:]]$MNT[[:space:]]" /etc/fstab | grep -q usrquota; then
      log "Ajout usrquota dans /etc/fstab pour $MNT (applique au prochain boot)"
      # Best effort : append option on matching line
      python3 - <<PY || true
import pathlib
p = pathlib.Path("/etc/fstab")
lines = p.read_text().splitlines()
mnt = "$MNT"
out = []
changed = False
for line in lines:
    if line.strip().startswith("#") or not line.strip():
        out.append(line)
        continue
    parts = line.split()
    if len(parts) >= 4 and parts[1] == mnt and "usrquota" not in parts[3]:
        parts[3] = parts[3] + ",usrquota"
        out.append("\t".join(parts))
        changed = True
    else:
        out.append(line)
if changed:
    p.write_text("\n".join(out) + "\n")
    print("fstab mis à jour")
else:
    print("fstab non modifié automatiquement — ajoutez usrquota à la main")
PY
      mount -o remount,usrquota "$MNT" 2>/dev/null || log "Remount impossible maintenant — reboot après fstab"
    fi
  fi
fi

log "quotacheck + quotaon…"
quotacheck -cum "$MNT" 2>/dev/null || quotacheck -cum / 2>/dev/null || true
quotaon -ug "$MNT" 2>/dev/null || quotaon -a 2>/dev/null || true

if quotaon -p "$MNT" 2>/dev/null | grep -qi "user quota on"; then
  log "Quotas utilisateur actifs sur $MNT"
else
  log "Quotas non confirmés — FS lab sans usrquota : setquota restera non bloquant"
fi
