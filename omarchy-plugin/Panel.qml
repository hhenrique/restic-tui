import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Commons as Commons
import qs.Ui
import "Model.js" as Model

Panel {
  id: root
  moduleName: "restic.backup-status"
  ipcTarget: "restic.backup-status"
  manageIpc: false

  readonly property color foreground: bar ? bar.foreground : Commons.Color.foreground
  readonly property color urgent: bar ? bar.urgent : Commons.Color.urgent
  readonly property color dim: Qt.darker(foreground, 1.55)
  readonly property color track: Style.selectedFillFor(foreground, Commons.Color.accent)
  readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family
  // Material "backup restore" glyph (U+F006F), built at runtime so the file stays ASCII.
  readonly property string glyph: String.fromCodePoint(0xF006F)

  // Age and state read this instead of Date.now() so the panel keeps telling
  // the truth while it sits open.
  property double nowMs: Date.now()
  readonly property var status: backup.status
  readonly property string state: Model.backupState(status, nowMs / 1000)
  readonly property bool alarming: state === "failed" || state === "stale"
  // The helper returns oldest-first; the panel lists newest-first.
  readonly property var days: status ? status.days.slice().reverse() : []
  readonly property var last: status ? status.last : null
  readonly property real peak: {
    var p = 0
    for (var i = 0; i < days.length; i++) p = Math.max(p, Number(days[i].dataAdded || 0))
    return Math.max(1, p)
  }

  readonly property string heroMeta: {
    var epoch = Model.lastEpoch(status)
    var age = epoch > 0 ? Model.formatAge(nowMs / 1000 - epoch) : ""
    if (state === "none") return backup.refreshing ? "Checking repository…" : "No backup recorded yet"
    if (state === "failed") return "Last run failed" + (age ? " · last good " + age : "")
    if (state === "stale") return "Overdue · last backup " + age
    return "Last backup " + age
  }

  function clamp(v, lo, hi) { return Math.max(lo, Math.min(hi, v)) }
  function alpha(c, a) { return Qt.rgba(c.r, c.g, c.b, a) }

  function refreshNow() { backup.refresh(false) }

  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  onOpenedChanged: if (opened) {
    nowMs = Date.now()
    backup.refresh(true)
    Qt.callLater(function() { keyCatcher.forceActiveFocus() })
  }

  Timer {
    interval: 30000
    repeat: true
    running: true
    onTriggered: root.nowMs = Date.now()
  }

  Service {
    id: backup
    settings: root.settings
    pluginDir: Qt.resolvedUrl(".").toString().replace(/^file:\/\//, "").replace(/\/$/, "")
  }

  IpcHandler {
    target: root.ipcTarget
    function open(): void { root.open() }
    function close(): void { root.close() }
    function show(): void { root.open() }
    function hide(): void { root.close() }
    function toggle(): void { root.toggle() }
    function refresh(): string { root.refreshNow(); return "ok" }
    function status(): string { return root.state + " · " + root.heroMeta }
  }

  BarIconButton {
    id: button
    anchors.fill: parent
    bar: root.bar
    text: root.glyph
    foreground: root.alarming ? root.urgent : root.barForeground
    onPressed: function(buttonCode) {
      if (buttonCode === Qt.RightButton) root.refreshNow()
      else root.toggle()
    }
  }

  KeyboardPanel {
    id: panel
    anchorItem: button
    owner: root
    bar: root.bar
    open: root.opened
    focusTarget: keyCatcher
    contentWidth: panel.fittedContentWidth(Style.space(360))
    contentHeight: panel.fittedContentHeight(column.implicitHeight, Style.space(560))

    PanelKeyCatcher {
      id: keyCatcher
      anchors.fill: parent
      onCloseRequested: root.close()
      onTabRequested: function(direction) { root.switchPanel(direction) }
      onTextKey: function(t) { if (t === "r" || t === "R") root.refreshNow() }

      Column {
        id: column
        width: parent.width
        spacing: Style.space(12)

        // ---------- Hero ----------
        PanelHero {
          id: hero
          width: parent.width
          title: "Restic backup"
          meta: root.heroMeta
          foreground: root.alarming ? root.urgent : root.foreground
          fontFamily: root.fontFamily
          iconComponent: Component {
            Text {
              textFormat: Text.PlainText
              text: root.glyph
              color: hero.foreground
              font.family: root.fontFamily
              font.pixelSize: Style.font.display
            }
          }
        }

        Text {
          textFormat: Text.PlainText
          visible: backup.lastError !== ""
          width: parent.width
          text: backup.lastError + (root.status && root.status.hasData ? " — showing cached data" : "")
          color: root.urgent
          font.family: root.fontFamily
          font.pixelSize: Style.font.bodySmall
          wrapMode: Text.WordWrap
        }

        Text {
          textFormat: Text.PlainText
          visible: !root.status || !root.status.hasData
          width: parent.width
          topPadding: Style.space(12)
          text: backup.refreshing ? "Reading the repository…" : "No snapshot data yet.\nThe first refresh opens the repository and can take a few seconds."
          color: root.dim
          font.family: root.fontFamily
          font.pixelSize: Style.font.body
          horizontalAlignment: Text.AlignHCenter
          wrapMode: Text.WordWrap
        }

        // ---------- Last snapshot ----------
        Column {
          id: snapshotSection
          visible: !!root.last
          width: parent.width
          spacing: Style.spacing.md

          PanelSectionHeader {
            width: parent.width
            text: "LAST SNAPSHOT"
            foreground: root.foreground
            fontFamily: root.fontFamily
          }

          Item {
            width: parent.width
            implicitHeight: Math.max(snapWhen.implicitHeight, snapAdded.implicitHeight)

            Text {
              id: snapWhen
              textFormat: Text.PlainText
              text: root.last ? Model.formatDateTime(root.last.epoch) : ""
              color: root.foreground
              font.family: root.fontFamily
              font.pixelSize: Style.font.body
              anchors.left: parent.left
              anchors.verticalCenter: parent.verticalCenter
            }

            Text {
              id: snapAdded
              textFormat: Text.PlainText
              text: root.last && root.last.hasSummary ? "+" + Model.formatBytes(root.last.dataAdded) : ""
              color: root.foreground
              font.family: root.fontFamily
              font.pixelSize: Style.font.caption
              font.bold: true
              anchors.right: parent.right
              anchors.verticalCenter: parent.verticalCenter
            }
          }

          Text {
            textFormat: Text.PlainText
            width: parent.width
            text: root.last
              ? [root.last.hostname, root.last.paths.join(", "), root.last.id].filter(function(s) { return s !== "" }).join(" · ")
              : ""
            color: root.dim
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
            elide: Text.ElideRight
          }
        }

        PanelSeparator {
          visible: weekSection.visible
          foreground: root.foreground
        }

        // ---------- Last 7 days ----------
        Column {
          id: weekSection
          visible: root.days.length > 0
          width: parent.width
          spacing: Style.spacing.md

          PanelSectionHeader {
            width: parent.width
            text: "LAST 7 DAYS"
            foreground: root.foreground
            fontFamily: root.fontFamily
          }

          Repeater {
            model: root.days

            DayRow {
              required property var modelData
              required property int index
              width: weekSection.width
              day: modelData
              // Square-root scale: a 40 MB day stays visible next to a 4 GB one.
              ratio: Math.sqrt(Number(modelData.dataAdded || 0) / root.peak)
              today: Model.daysAgo(modelData.date, root.nowMs) <= 0
            }
          }
        }

        Text {
          textFormat: Text.PlainText
          visible: !!root.status && root.status.hasData
          width: parent.width
          topPadding: Style.space(2)
          text: root.status ? root.status.snapshotCount + " snapshots in repository" : ""
          color: root.dim
          font.family: root.fontFamily
          font.pixelSize: Style.font.caption
          horizontalAlignment: Text.AlignHCenter
        }
      }
    }
  }

  // One row per day: weekday, bar scaled to the week's biggest day, size added.
  // Days with no snapshot get an empty ring and "none"; today is picked out in
  // full foreground so the week reads as a run-up to right now.
  component DayRow: Item {
    id: dayRow
    property var day: null
    property real ratio: 0
    property bool today: false
    readonly property bool empty: !day || !day.count

    implicitHeight: Math.max(dayLabel.implicitHeight, dayValue.implicitHeight) + Style.spacing.sm

    Text {
      id: dayLabel
      textFormat: Text.PlainText
      text: dayRow.day ? Model.dayLabel(dayRow.day.date, root.nowMs) : ""
      color: dayRow.today ? root.foreground : root.dim
      font.family: root.fontFamily
      font.pixelSize: Style.font.caption
      font.bold: dayRow.today
      anchors.left: parent.left
      anchors.verticalCenter: parent.verticalCenter
      width: Style.space(78)
    }

    Rectangle {
      id: dayTrack
      anchors.left: dayLabel.right
      anchors.right: dayValue.left
      anchors.leftMargin: Style.space(8)
      anchors.rightMargin: Style.space(10)
      anchors.verticalCenter: parent.verticalCenter
      height: Math.max(Style.space(4), Math.round(Style.spacing.controlHeight * 0.14))
      radius: height / 2
      color: root.track

      Rectangle {
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        height: parent.height
        radius: parent.radius
        visible: !dayRow.empty
        // A day with a snapshot never rounds down to an empty-looking track.
        width: Math.max(parent.height, parent.width * root.clamp(dayRow.ratio, 0, 1))
        color: dayRow.today ? root.foreground : root.alpha(root.foreground, 0.55)

        Behavior on width {
          NumberAnimation { duration: 160; easing.type: Easing.OutCubic }
        }
      }
    }

    Text {
      id: dayValue
      textFormat: Text.PlainText
      text: dayRow.empty ? "none" : "+" + Model.formatBytes(dayRow.day.dataAdded)
      color: dayRow.empty ? root.dim : (dayRow.today ? root.foreground : root.dim)
      font.family: root.fontFamily
      font.pixelSize: Style.font.caption
      font.bold: !dayRow.empty
      horizontalAlignment: Text.AlignRight
      anchors.right: parent.right
      anchors.verticalCenter: parent.verticalCenter
      width: Style.space(64)
    }

    MouseArea {
      id: dayHover
      anchors.fill: parent
      hoverEnabled: true
      acceptedButtons: Qt.NoButton
    }

    PanelToolTip {
      visible: dayHover.containsMouse
      text: Model.dayTooltip(dayRow.day)
      fontFamily: root.fontFamily
    }
  }
}
