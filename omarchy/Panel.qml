import QtQuick
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Commons as Commons
import qs.Ui
import "Model.js" as Model

// The list of open todos and the quick-capture field. Everything goes through
// the tlog command line — `tlog todo -json` to read, `tlog block <addr> task`
// to tick off, `tlog add -todo` to capture — so this widget never parses a note
// and never writes one itself. Each address carries the hash of the file it was
// read from, so ticking off a task that changed in the meantime is refused by
// tlog (exit 4) rather than landing on the wrong line.
Panel {
  id: root
  moduleName: "io.github.tilman-schieber.tlog"
  ipcTarget: "io.github.tilman-schieber.tlog"
  manageIpc: false

  property var anchorItem: null
  property var hostWidget: null
  readonly property var barIdentity: hostWidget || root

  readonly property string tlog: setting("tlogPath", "") || "tlog"

  property var items: []
  readonly property var listRows: Model.rows(items)
  readonly property var counts: Model.summary(items)
  readonly property string barState: Model.barState(counts)
  readonly property string tooltip: counts.open === 0 ? "Nothing open"
    : [counts.overdue ? counts.overdue + " overdue" : "",
       counts.today ? counts.today + " due today" : "",
       counts.open + " open"].filter(function(s) { return s !== "" }).join(" · ")

  property int selected: -1
  property string todayIso: Qt.formatDate(new Date(), "yyyy-MM-dd")
  property string todayPath: ""
  property string message: ""
  property bool messageBad: false
  // Rows being ticked off, by address: they strike through and fade while
  // tlog writes, and are gone at the next refresh.
  property var ticking: ({})
  property bool justReturned: false

  readonly property color fg: root.bar ? root.bar.barForeground : Commons.Color.foreground
  readonly property color muted: Qt.darker(fg, 1.5)
  readonly property string fontFamily: root.bar ? root.bar.fontFamily : Style.font.family

  function stateColor(key) {
    if (key === "overdue") return Commons.Color.urgent
    if (key === "today") return Commons.Color.accent
    return root.muted
  }

  // ---- open and close -----------------------------------------------------

  function open() { show(false) }
  function openFromHotkey() { show(false) }
  function openCapture() { show(true) }

  function show(capture) {
    root.controller.show()
    root.refresh()
    if (capture) {
      Qt.callLater(function() { field.forceActiveFocus() })
    } else {
      selected = Model.firstSelectable(listRows)
      Qt.callLater(function() { keyCatcher.forceActiveFocus() })
    }
  }

  function close() {
    field.text = ""
    root.controller.hide()
  }

  function toggle() {
    if (root.opened) root.close()
    else root.show(false)
  }

  function switchPanel(direction) {
    if (root.bar && typeof root.bar.switchPanelFrom === "function")
      return root.bar.switchPanelFrom(root.barIdentity, direction)
    return false
  }

  function say(text, bad) {
    message = text
    messageBad = !!bad
    messageTimer.restart()
  }

  // ---- reading ------------------------------------------------------------

  property bool refreshQueued: false

  function refresh() {
    if (listProc.running) { refreshQueued = true; return }
    listProc.running = true
  }

  Process {
    id: listProc
    command: [root.tlog, "todo", "-json"]
    stdout: StdioCollector {
      waitForEnd: true
      onStreamFinished: {
        try {
          root.items = JSON.parse(String(text || "[]")) || []
          root.ticking = ({})
          if (root.selected >= root.listRows.length || (root.listRows[root.selected] && root.listRows[root.selected].header))
            root.selected = Model.firstSelectable(root.listRows)
        } catch (e) {
          // Keep the last good list rather than emptying the bar.
        }
      }
    }
    stderr: StdioCollector { id: listErr; waitForEnd: true }
    onExited: function(code) {
      if (code !== 0) root.say(String(listErr.text || "").trim() || "tlog todo failed — is tlog on PATH?", true)
      if (root.refreshQueued) {
        root.refreshQueued = false
        Qt.callLater(root.refresh)
      }
    }
  }

  // Where today's journal is, so an edit to it — in the outliner, in nvim, from
  // `tlog add` — shows up here at once rather than at the next minute.
  Process {
    id: todayProc
    command: [root.tlog, "today"]
    stdout: StdioCollector {
      waitForEnd: true
      onStreamFinished: root.todayPath = String(text || "").trim()
    }
  }

  FileView {
    path: root.todayPath
    watchChanges: true
    printErrors: false
    onFileChanged: { reload(); root.refresh() }
  }

  // Everything else — a page edited elsewhere, a deadline that became overdue
  // at midnight — is caught by looking again every minute. A new day also means
  // a new journal to watch.
  Timer {
    interval: 60 * 1000
    running: true
    repeat: true
    onTriggered: {
      var now = Qt.formatDate(new Date(), "yyyy-MM-dd")
      if (now !== root.todayIso) {
        root.todayIso = now
        todayProc.running = true
      }
      root.refresh()
    }
  }

  Component.onCompleted: {
    todayProc.running = true
    refresh()
  }

  // ---- writing ------------------------------------------------------------

  property var queue: []

  // run hands one command to tlog at a time, in order: ticking two things off
  // quickly must not race two writes to the same journal.
  function run(args, done) {
    queue = queue.concat([{ args: args, done: done }])
    if (!actionProc.running) next()
  }

  function next() {
    if (queue.length === 0) return
    var job = queue[0]
    queue = queue.slice(1)
    actionProc.done = job.done
    actionProc.command = job.args
    actionProc.running = true
  }

  Process {
    id: actionProc
    property var done: null
    stderr: StdioCollector { id: actionErr; waitForEnd: true }
    onExited: function(code) {
      if (actionProc.done) actionProc.done(code, String(actionErr.text || "").trim())
      Qt.callLater(root.next)
    }
  }

  function tick(item) {
    if (!item || ticking[item.addr]) return
    var t = Object.assign({}, ticking)
    t[item.addr] = true
    ticking = t
    run([root.tlog, "block", item.addr, "task"], function(code, err) {
      if (code === 4) root.say("That changed on disk — here is the list as it is now.", true)
      else if (code !== 0) root.say(err || "could not tick it off", true)
      // Let the strike-through be seen before the row goes.
      refreshAfterFade.restart()
    })
  }

  Timer {
    id: refreshAfterFade
    interval: 450
    onTriggered: root.refresh()
  }

  function capture(asNote) {
    var args = Model.captureArgs(root.tlog, field.text, asNote)
    if (!args) return
    var typed = field.text
    field.text = ""
    run(args, function(code, err) {
      if (code !== 0) {
        field.text = typed // nothing was written; nothing typed is lost
        root.say(err.replace(/^tlog( add)?: /, "") || "could not add it", true)
        return
      }
      root.say(asNote ? "Noted in today's journal." : "Added to today's journal.", false)
      root.refresh()
    })
  }

  // ---- going to tlog -----------------------------------------------------

  function openJournal() {
    Quickshell.execDetached(["omarchy-launch-or-focus-tui", root.tlog])
    root.close()
  }

  function openItem(item) {
    if (!item) return
    // A journal's page name is its date, which tlog resolves to that day.
    Quickshell.execDetached(["omarchy-launch-tui", "--app-id=org.omarchy.tlog-page", root.tlog, "edit", item.page])
    root.close()
  }

  // ---- ipc ----------------------------------------------------------------

  IpcHandler {
    target: root.ipcTarget

    function open(): void { root.show(false) }
    function show(): void { root.show(false) }
    function close(): void { root.close() }
    function hide(): void { root.close() }
    function toggle(): void { root.toggle() }
    function capture(): void { root.openCapture() }
    function journal(): void { root.openJournal() }
    function refresh(): void { root.refresh() }
  }

  Timer {
    id: messageTimer
    interval: 4000
    onTriggered: root.message = ""
  }

  // ---- the panel ----------------------------------------------------------

  KeyboardPanel {
    id: panel
    anchorItem: root.anchorItem
    owner: root.barIdentity
    bar: root.bar
    open: root.opened
    focusTarget: keyCatcher
    contentWidth: panel.fittedContentWidth(Style.space(440))
    contentHeight: panel.fittedContentHeight(content.implicitHeight, Style.space(620))

    PanelKeyCatcher {
      id: keyCatcher
      anchors.fill: parent
      blocked: field.activeFocus

      onCloseRequested: root.close()
      onTabRequested: function(direction) { root.switchPanel(direction) }
      onMoveRequested: function(dx, dy) {
        if (dy === 0) return
        root.selected = root.selected < 0 ? Model.firstSelectable(root.listRows)
          : Model.nextSelectable(root.listRows, root.selected, dy)
        list.positionViewAtSelected()
      }
      // Enter opens the page; space and x tick off. Enter emits both signals,
      // return first, so the activate that follows it is skipped.
      onReturnRequested: {
        root.justReturned = true
        var r = root.listRows[root.selected]
        if (r && !r.header) root.openItem(r.item)
      }
      onActivateRequested: {
        if (root.justReturned) { root.justReturned = false; return }
        var r = root.listRows[root.selected]
        if (r && !r.header) root.tick(r.item)
      }
      onDeleteRequested: {
        var r = root.listRows[root.selected]
        if (r && !r.header) root.tick(r.item)
      }
      onTextKey: function(t) {
        if (t === "a" || t === "n" || t === "+" || t === "/") field.forceActiveFocus()
        else if (t === "t") root.openJournal()
        else if (t === "r") root.refresh()
      }

      Column {
        id: content
        width: parent.width
        spacing: Style.space(12)

        // ---- the day, and what it holds
        Item {
          width: parent.width
          height: Math.max(dayColumn.height, journalButton.height)

          Column {
            id: dayColumn
            anchors.left: parent.left
            anchors.leftMargin: Style.space(4)
            anchors.verticalCenter: parent.verticalCenter
            spacing: Style.space(2)

            Text {
              text: Qt.formatDate(new Date(root.todayIso + "T12:00:00"), "dddd")
              color: root.fg
              font.family: root.fontFamily
              font.pixelSize: Style.font.title
              font.bold: true
            }
            Row {
              spacing: Style.space(10)
              Text {
                text: Qt.formatDate(new Date(root.todayIso + "T12:00:00"), "d MMMM")
                color: root.muted
                font.family: root.fontFamily
                font.pixelSize: Style.font.bodySmall
              }
              Repeater {
                model: [
                  { n: root.counts.overdue, label: "overdue", key: "overdue" },
                  { n: root.counts.today, label: "today", key: "today" },
                  { n: root.counts.open, label: "open", key: "" }
                ].filter(function(c) { return c.n > 0 })
                Text {
                  required property var modelData
                  text: "● " + modelData.n + " " + modelData.label
                  color: modelData.key ? root.stateColor(modelData.key) : root.muted
                  font.family: root.fontFamily
                  font.pixelSize: Style.font.bodySmall
                }
              }
            }
          }

          Rectangle {
            id: journalButton
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            width: journalLabel.implicitWidth + Style.space(20)
            height: journalLabel.implicitHeight + Style.space(12)
            radius: Style.cornerRadius
            color: journalArea.containsMouse ? Style.hoverFillFor(root.fg, Commons.Color.accent) : "transparent"
            border.width: Math.max(1, Style.spacing.hairline)
            border.color: Qt.rgba(root.fg.r, root.fg.g, root.fg.b, 0.18)

            Text {
              id: journalLabel
              anchors.centerIn: parent
              text: "󰠮  Journal"   // nf-md-notebook
              color: journalArea.containsMouse ? Style.hoverStateColor(root.fg, Commons.Color.accent) : root.fg
              font.family: root.fontFamily
              font.pixelSize: Style.font.bodySmall
            }
            MouseArea {
              id: journalArea
              anchors.fill: parent
              hoverEnabled: true
              cursorShape: Qt.PointingHandCursor
              onClicked: root.openJournal()
            }
          }
        }

        // ---- capture
        TextField {
          id: field
          width: parent.width
          placeholderText: "Add a todo  ·  @fri for a deadline"
          foreground: root.fg
          font.family: root.fontFamily

          Keys.onPressed: function(event) {
            if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
              root.capture((event.modifiers & Qt.ShiftModifier) !== 0)
              event.accepted = true
            } else if (event.key === Qt.Key_Escape) {
              if (field.text !== "") field.text = ""
              else root.close()
              event.accepted = true
            } else if (event.key === Qt.Key_Down || event.key === Qt.Key_Tab) {
              root.selected = Model.firstSelectable(root.listRows)
              keyCatcher.forceActiveFocus()
              event.accepted = true
            }
          }
        }

        Rectangle {
          width: parent.width
          height: Style.spacing.hairline
          color: root.fg
          opacity: 0.12
        }

        // ---- nothing to do
        Column {
          visible: root.items.length === 0
          width: parent.width
          spacing: Style.space(6)
          topPadding: Style.space(10)
          bottomPadding: Style.space(10)

          Text {
            anchors.horizontalCenter: parent.horizontalCenter
            text: "󰄵"
            color: Commons.Color.accent
            font.family: root.fontFamily
            font.pixelSize: Style.font.display
          }
          Text {
            anchors.horizontalCenter: parent.horizontalCenter
            text: "Nothing open."
            color: root.muted
            font.family: root.fontFamily
            font.pixelSize: Style.font.body
          }
        }

        // ---- the list
        ListView {
          id: list
          visible: root.items.length > 0
          width: parent.width
          height: Math.min(contentHeight, Style.space(440))
          clip: true
          interactive: contentHeight > height
          boundsBehavior: Flickable.StopAtBounds
          model: root.listRows
          spacing: Style.space(2)

          function positionViewAtSelected() {
            if (root.selected >= 0) positionViewAtIndex(root.selected, ListView.Contain)
          }

          delegate: Item {
            id: rowItem
            required property var modelData
            required property int index
            readonly property bool isHeader: modelData.header
            readonly property var item: modelData.item
            readonly property bool isTicking: !isHeader && root.ticking[item.addr] === true
            readonly property bool isSelected: index === root.selected

            width: ListView.view.width
            height: isHeader ? headerText.implicitHeight + Style.space(isHeader && index > 0 ? 14 : 4)
                             : taskColumn.implicitHeight + Style.space(12)

            // A group heading: the state's colour, so overdue reads as overdue
            // before a word of it is read.
            Text {
              id: headerText
              visible: rowItem.isHeader
              anchors.left: parent.left
              anchors.leftMargin: Style.space(4)
              anchors.bottom: parent.bottom
              anchors.bottomMargin: Style.space(2)
              text: rowItem.isHeader ? rowItem.modelData.title.toUpperCase() + "  " + rowItem.modelData.count : ""
              color: root.stateColor(rowItem.modelData.key)
              font.family: root.fontFamily
              font.pixelSize: Style.font.caption
              font.letterSpacing: 1
              font.bold: true
            }

            Rectangle {
              visible: !rowItem.isHeader
              anchors.fill: parent
              radius: Style.cornerRadius
              color: rowItem.isSelected || rowArea.containsMouse ? Style.hoverFillFor(root.fg, Commons.Color.accent) : "transparent"
              opacity: rowItem.isTicking ? 0.35 : 1
              Behavior on opacity { NumberAnimation { duration: 300; easing.type: Easing.OutCubic } }

              MouseArea {
                id: rowArea
                anchors.fill: parent
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onEntered: root.selected = rowItem.index
                onClicked: root.openItem(rowItem.item)
              }

              // The checkbox: a ring in the state's colour that fills when
              // it is ticked.
              Rectangle {
                id: box
                anchors.left: parent.left
                anchors.leftMargin: Style.space(8)
                anchors.top: parent.top
                anchors.topMargin: Style.space(8)
                width: Style.space(14)
                height: width
                radius: width / 2
                color: rowItem.isTicking ? root.stateColor(rowItem.modelData.key === "" ? "today" : rowItem.modelData.key) : "transparent"
                border.width: Math.max(1, Style.space(2))
                border.color: boxArea.containsMouse ? Commons.Color.accent
                  : root.stateColor(rowItem.modelData.key === "" ? "" : rowItem.modelData.key)
                Behavior on color { ColorAnimation { duration: 200 } }

                Text {
                  anchors.centerIn: parent
                  visible: rowItem.isTicking || boxArea.containsMouse
                  text: "✓"
                  color: rowItem.isTicking ? Commons.Color.background : Commons.Color.accent
                  font.pixelSize: Style.font.caption
                  font.bold: true
                }

                MouseArea {
                  id: boxArea
                  anchors.fill: parent
                  anchors.margins: -Style.space(6)
                  hoverEnabled: true
                  cursorShape: Qt.PointingHandCursor
                  onClicked: root.tick(rowItem.item)
                }
              }

              Column {
                id: taskColumn
                anchors.left: box.right
                anchors.leftMargin: Style.space(10)
                anchors.right: parent.right
                anchors.rightMargin: Style.space(8)
                anchors.verticalCenter: parent.verticalCenter
                spacing: Style.space(2)

                Text {
                  width: parent.width
                  text: rowItem.isHeader ? "" : Model.plain(rowItem.item.text)
                  color: root.fg
                  font.family: root.fontFamily
                  font.pixelSize: Style.font.body
                  font.strikeout: rowItem.isTicking
                  elide: Text.ElideRight
                  maximumLineCount: 2
                  wrapMode: Text.Wrap
                }
                Text {
                  width: parent.width
                  text: rowItem.isHeader ? "" : [Model.pageLabel(rowItem.item, root.todayIso), rowItem.item.label]
                    .filter(function(s) { return s }).join("  ·  ")
                  color: rowItem.modelData.key === "overdue" ? Commons.Color.urgent : root.muted
                  font.family: root.fontFamily
                  font.pixelSize: Style.font.caption
                  elide: Text.ElideRight
                }
              }
            }
          }
        }

        // ---- what just happened, and the keys
        Text {
          width: parent.width
          text: root.message !== "" ? root.message
            : "↑↓ choose · space tick off · enter open · a add · t journal"
          color: root.message !== "" && root.messageBad ? Commons.Color.urgent : root.muted
          font.family: root.fontFamily
          font.pixelSize: Style.font.caption
          horizontalAlignment: Text.AlignHCenter
          elide: Text.ElideRight
        }
      }
    }
  }
}
