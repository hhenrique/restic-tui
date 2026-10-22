# Restic backup — Omarchy bar widget

Bar icon + panel showing the last restic snapshot (time, data added, host,
paths, id) and a 7-day view of how much each day's snapshots added. The icon
turns red when the backup is overdue or the last run failed.

- `status.py` runs `restic snapshots --json` (credentials from the environment
  or `~/.restic-env`), caches the result in `~/.cache/restic-backup-plugin/`,
  and reads the `last-success` / `last-failure` stamps from
  `~/.local/state/restic-backup/` (see `scripts/backup-due-check`).
- The panel opens on cached data instantly; a live refresh runs every
  `refreshIntervalSec` (default 15 min) because opening the repo costs a KDF
  plus a network round trip. Right-click the icon or press `r` to refresh now.

## Install

```sh
cp -r omarchy-plugin ~/.config/omarchy/plugins/restic.backup-status
omarchy-shell shell rescanPlugins
omarchy plugin enable restic.backup-status
```
