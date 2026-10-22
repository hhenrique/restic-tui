import QtQuick
import Quickshell
import Quickshell.Io
import qs.Commons
import "Model.js" as Model

Item {
  id: root

  property var settings: ({})
  property string pluginDir: ""

  property var status: null
  property bool refreshing: false
  property string lastError: ""

  readonly property int refreshIntervalSec: intSetting("refreshIntervalSec", 900, 60, 86400)
  readonly property int staleAfterDays: intSetting("staleAfterDays", 3, 1, 30)
  readonly property string envFile: String(setting("envFile", ""))
  readonly property string helperPath: pluginDir === "" ? "" : pluginDir + "/status.py"

  property string _output: ""
  property string _error: ""

  function setting(name, fallback) {
    var value = settings ? settings[name] : undefined
    return value === undefined || value === null ? fallback : value
  }

  function intSetting(name, fallback, min, max) {
    var n = parseInt(String(setting(name, fallback)), 10)
    if (!isFinite(n)) n = fallback
    return Math.max(min, Math.min(max, n))
  }

  // cacheOnly reads the last result without opening the repository: instant,
  // so the panel and bar icon have something to show right away.
  function refresh(cacheOnly) {
    if (process.running || helperPath === "") return
    _output = ""
    _error = ""
    var command = ["python3", helperPath, "--stale-days", String(staleAfterDays)]
    if (envFile !== "") command.push("--env-file", envFile)
    if (cacheOnly) command.push("--cache-only")
    else refreshing = true
    process.command = command
    process.running = true
  }

  function apply(raw) {
    var parsed = Model.parseStatus(raw)
    if (!parsed) {
      lastError = "Could not read backup status"
      return
    }
    status = parsed
    lastError = parsed.error ? String(parsed.error) : ""
  }

  Timer {
    interval: root.refreshIntervalSec * 1000
    repeat: true
    running: root.helperPath !== ""
    onTriggered: root.refresh(false)
  }

  // Show cached data immediately, then go live once the shell has settled.
  Timer {
    interval: 1
    running: root.helperPath !== ""
    onTriggered: root.refresh(true)
  }

  Timer {
    id: firstLive
    interval: 15000
    running: root.helperPath !== ""
    onTriggered: root.refresh(false)
  }

  Process {
    id: process
    running: false
    command: []
    stdout: StdioCollector { id: stdoutCollector; waitForEnd: true; onStreamFinished: root._output = text }
    stderr: StdioCollector { id: stderrCollector; waitForEnd: true; onStreamFinished: root._error = text }
    onExited: function(exitCode) {
      root.refreshing = false
      var out = String(stdoutCollector.text || root._output || "")
      var err = String(stderrCollector.text || root._error || "")
      if (exitCode === 0) root.apply(out)
      else root.lastError = (err.trim().split("\n").pop() || "Backup status helper failed").substring(0, 160)
    }
  }
}
