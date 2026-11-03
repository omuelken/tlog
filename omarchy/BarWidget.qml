import QtQuick
import qs.Commons
import qs.Commons as Commons
import qs.Ui

// The pill: a notebook and how many things are open, in the colour of the most
// pressing one — urgent when something is overdue, the accent when something is
// due today. The list itself, and everything that talks to tlog, is Panel.qml.
BarWidget {
  id: root
  moduleName: "io.github.tilman-schieber.tlog"

  readonly property var panel: panelLoader.item
  readonly property var counts: panel ? panel.counts : ({ open: 0, overdue: 0, today: 0 })
  readonly property string barState: panel ? panel.barState : "clear"

  function injectPanel() {
    var target = panelLoader.item
    if (!target) return
    if ("bar" in target) target.bar = root.bar
    if ("settings" in target) target.settings = root.settings
    if ("anchorItem" in target) target.anchorItem = button
    if ("hostWidget" in target) target.hostWidget = root
  }

  // The shape Bar.findPanelWidget expects, so `shell summon` and switching
  // between panels with tab find this widget.
  readonly property bool opened: panel ? panel.opened === true : false
  readonly property bool popoutSwitchClosing: panel ? panel.popoutSwitchClosing === true : false
  function open() { if (panel) panel.openFromHotkey() }
  function close() { if (panel) panel.close() }
  function closeForPopoutSwitch() { if (panel) panel.closeForPopoutSwitch() }

  visible: !(setting("hideWhenClear", false) && barState === "clear")
  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  onBarChanged: injectPanel()
  onSettingsChanged: injectPanel()

  Loader {
    id: panelLoader
    active: true
    source: Qt.resolvedUrl("Panel.qml")
    visible: false
    onLoaded: {
      root.injectPanel()
      Qt.callLater(root.injectPanel)
    }
  }

  WidgetButton {
    id: button
    anchors.fill: parent
    bar: root.bar
    useActiveColor: false
    // nf-md-notebook, the same mark as the panel's journal button; the count
    // beside it goes when there is nothing left to do.
    text: root.barState === "clear" ? "󰠮" : "󰠮 " + root.counts.open
    foreground: root.barState === "overdue" ? Commons.Color.urgent
      : root.barState === "today" ? Commons.Color.accent
      : root.barState === "clear" ? Commons.Color.muted
      : (root.bar ? root.bar.barForeground : Commons.Color.foreground)
    tooltipText: root.opened ? "" : (root.panel ? root.panel.tooltip : "")

    onPressed: function(b) {
      if (!root.panel) return
      if (b === Qt.RightButton) root.panel.openJournal()
      else if (b === Qt.MiddleButton) root.panel.openCapture()
      else root.panel.toggle()
    }
  }
}
