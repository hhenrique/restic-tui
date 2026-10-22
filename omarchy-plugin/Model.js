// Same thresholds as the TUI heatmap (heatmap.go bucketForBytes): this is a
// heatmap, not a precise chart. 0 = no backup that day, 1..4 = data added.
function levelForBytes(bytes, count) {
  if (!count) return 0
  if (bytes < 50 * 1024 * 1024) return 1
  if (bytes < 500 * 1024 * 1024) return 2
  if (bytes < 5 * 1024 * 1024 * 1024) return 3
  return 4
}

function formatBytes(bytes) {
  var n = Number(bytes || 0)
  var units = ["B", "KB", "MB", "GB", "TB"]
  var i = 0
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i++ }
  var text = i === 0 || n >= 100 ? String(Math.round(n)) : n.toFixed(1)
  return text + " " + units[i]
}

function formatAge(seconds) {
  var s = Math.max(0, Math.floor(Number(seconds || 0)))
  if (s < 90) return "just now"
  var m = Math.floor(s / 60)
  if (m < 60) return m + "m ago"
  var h = Math.floor(m / 60)
  if (h < 48) return h + "h ago"
  return Math.floor(h / 24) + "d ago"
}

function formatDateTime(epoch) {
  if (!epoch) return "—"
  return Qt.formatDateTime(new Date(epoch * 1000), "ddd d MMM, HH:mm")
}

function parseStatus(raw) {
  var text = String(raw || "").trim()
  if (text === "") return null
  try {
    var parsed = JSON.parse(text)
    if (!parsed || typeof parsed !== "object") return null
    parsed.days = Array.isArray(parsed.days) ? parsed.days : []
    return parsed
  } catch (e) {
    return null
  }
}

// "none" | "failed" | "stale" | "ok". A failed run only counts while nothing
// newer has succeeded — a snapshot or a manual `touch last-success` clears it.
function backupState(status, nowSec) {
  if (!status || !status.hasData) return status && status.lastSuccess ? "ok" : "none"
  var lastOk = Math.max(status.last ? status.last.epoch : 0, status.lastSuccess || 0)
  if (status.lastFailure && status.lastFailure > lastOk) return "failed"
  if (lastOk === 0) return "none"
  if (nowSec - lastOk >= status.staleAfterDays * 86400) return "stale"
  return "ok"
}

function lastEpoch(status) {
  if (!status) return 0
  return Math.max(status.last ? status.last.epoch : 0, status.lastSuccess || 0)
}

function dayTooltip(day) {
  if (!day) return ""
  if (!day.count) return day.date + " · no backup"
  return day.date + " · " + day.count + (day.count === 1 ? " snapshot" : " snapshots")
    + (day.dataAdded > 0 ? " · +" + formatBytes(day.dataAdded) : "")
}

// Days between a local YYYY-MM-DD date and now, by calendar date (not 24h
// spans), so "Yesterday" flips at midnight.
function daysAgo(dateText, nowMs) {
  var p = String(dateText).split("-")
  var then = new Date(Number(p[0]), Number(p[1]) - 1, Number(p[2]))
  var now = new Date(nowMs)
  var today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  return Math.round((today.getTime() - then.getTime()) / 86400000)
}

function dayLabel(dateText, nowMs) {
  var n = daysAgo(dateText, nowMs)
  if (n <= 0) return "Today"
  if (n === 1) return "Yesterday"
  return n + " days ago"
}
