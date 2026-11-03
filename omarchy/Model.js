.pragma library

// Pure helpers for the tlog bar widget, kept out of the QML so node can test
// them (tests/model_test.js). Everything here works on what
// `tlog todo -json` prints; nothing re-reads or re-parses the notes.

// The order a day's attention goes in. The states are the ones tlog itself
// decides (internal/dates.Status), so "soon" means the same here as in the
// outliner's agenda.
var GROUPS = [
  { key: "overdue", title: "Overdue" },
  { key: "today", title: "Today" },
  { key: "soon", title: "This week" },
  { key: "later", title: "Later" },
  { key: "", title: "Someday" }
]

function groups(items) {
  var out = []
  for (var g = 0; g < GROUPS.length; g++) {
    var members = []
    for (var i = 0; i < (items || []).length; i++) {
      if ((items[i].state || "") === GROUPS[g].key) members.push(items[i])
    }
    if (members.length > 0) out.push({ key: GROUPS[g].key, title: GROUPS[g].title, items: members })
  }
  return out
}

// rows flattens the groups into what the panel draws, a header before each
// group, so one index can walk the list from the keyboard.
function rows(items) {
  var out = []
  var gs = groups(items)
  for (var g = 0; g < gs.length; g++) {
    out.push({ header: true, title: gs[g].title, key: gs[g].key, count: gs[g].items.length })
    for (var i = 0; i < gs[g].items.length; i++) out.push({ header: false, item: gs[g].items[i], key: gs[g].key })
  }
  return out
}

function summary(items) {
  var s = { open: 0, overdue: 0, today: 0 }
  for (var i = 0; i < (items || []).length; i++) {
    s.open++
    if (items[i].state === "overdue") s.overdue++
    if (items[i].state === "today") s.today++
  }
  return s
}

// barState is what the pill says at a glance: whatever is most pressing.
function barState(s) {
  if (s.overdue > 0) return "overdue"
  if (s.today > 0) return "today"
  if (s.open > 0) return "open"
  return "clear"
}

// nextSelectable moves through rows, skipping headers.
function nextSelectable(list, from, delta) {
  var i = from
  for (var n = 0; n < list.length; n++) {
    i += delta
    if (i < 0 || i >= list.length) return from
    if (!list[i].header) return i
  }
  return from
}

function firstSelectable(list) {
  return nextSelectable(list, -1, 1)
}

// parseCapture splits what was typed into the text and a deadline written at
// the end after an @: "call the printer @fri", "send it @in 3 days". The
// deadline is handed to tlog as typed; tlog's own parser decides what it
// means, and refuses what it cannot read rather than guessing.
function parseCapture(raw) {
  var text = String(raw || "").trim()
  var at = text.lastIndexOf(" @")
  if (at < 0 && text.charAt(0) === "@") at = -1
  if (at >= 0) {
    var due = text.slice(at + 2).trim()
    var body = text.slice(0, at).trim()
    if (due !== "" && body !== "") return { text: body, due: due }
  }
  return { text: text, due: "" }
}

// captureArgs is the command that writes it: a task in today's journal, or a
// plain note when asked for one.
function captureArgs(tlog, raw, asNote) {
  var c = parseCapture(raw)
  if (c.text === "") return null
  if (asNote) return [tlog, "add", String(raw).trim()]
  var args = [tlog, "add", "-todo"]
  if (c.due !== "") args.push("-due", c.due)
  args.push(c.text)
  return args
}

// pageLabel is where a task lives, said briefly: a journal by its day, a page
// by its name.
function pageLabel(item, todayIso) {
  if (!item) return ""
  if (item.page === todayIso) return "today's journal"
  return item.page
}

// plain strips the markdown a single line of task text is likely to carry, so
// the list reads as prose: [[links]], [label](url), `code`, **bold**.
function plain(text) {
  return String(text || "")
    .replace(/!?\[([^\[\]]*)\]\([^()\s]+\)/g, "$1")
    .replace(/\[\[([^\[\]#]+)(?:#\^[^\]]*)?\]\]/g, "$1")
    .replace(/`([^`]+)`/g, "$1")
    .replace(/\*\*([^*]+)\*\*/g, "$1")
    .replace(/\s+/g, " ")
    .trim()
}
