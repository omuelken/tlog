// node omarchy/tests/model_test.js — the widget's pure logic, without Quickshell.
const fs = require("fs")
const path = require("path")
const vm = require("vm")

const src = fs.readFileSync(path.join(__dirname, "..", "Model.js"), "utf8").replace(/^\.pragma library\s*$/m, "")
const M = {}
vm.runInNewContext(src + "\nObject.assign(M, { groups, rows, summary, barState, nextSelectable, firstSelectable, parseCapture, captureArgs, pageLabel, plain })", { M })

let failed = 0
function eq(name, got, want) {
  const g = JSON.stringify(got), w = JSON.stringify(want)
  if (g !== w) { failed++; console.error(`FAIL ${name}\n  got  ${g}\n  want ${w}`) }
}

const items = [
  { text: "late", state: "overdue", page: "2026-10-01" },
  { text: "undated", state: "", page: "Plans" },
  { text: "now", state: "today", page: "2026-11-03" },
  { text: "also late", state: "overdue", page: "2026-10-02" },
]

eq("groups in order of attention", M.groups(items).map(g => [g.title, g.items.length]),
  [["Overdue", 2], ["Today", 1], ["Someday", 1]])
eq("rows put a header before each group", M.rows(items).map(r => r.header ? "#" + r.title : r.item.text),
  ["#Overdue", "late", "also late", "#Today", "now", "#Someday", "undated"])
eq("summary", M.summary(items), { open: 4, overdue: 2, today: 1 })
eq("the bar says the most pressing thing", ["overdue", "today", "open", "clear"].map((_, i) =>
  M.barState([{ open: 4, overdue: 2, today: 1 }, { open: 2, overdue: 0, today: 1 }, { open: 1, overdue: 0, today: 0 }, { open: 0, overdue: 0, today: 0 }][i])),
  ["overdue", "today", "open", "clear"])

const list = M.rows(items)
eq("the first selectable row skips the header", M.firstSelectable(list), 1)
eq("down skips the next header", M.nextSelectable(list, 2, 1), 4)
eq("up skips a header too", M.nextSelectable(list, 4, -1), 2)
eq("the ends hold", [M.nextSelectable(list, 1, -1), M.nextSelectable(list, 6, 1)], [1, 6])

eq("a deadline after @", M.parseCapture("call the printer @fri"), { text: "call the printer", due: "fri" })
eq("a deadline of several words", M.parseCapture("send it @in 3 days"), { text: "send it", due: "in 3 days" })
eq("an @ inside the text is text", M.parseCapture("mail a@b.ch about it"), { text: "mail a@b.ch about it", due: "" })
eq("only a deadline is not a task", M.parseCapture("@fri"), { text: "@fri", due: "" })

eq("a task with a deadline", M.captureArgs("tlog", " book the room @tomorrow ", false),
  ["tlog", "add", "-todo", "-due", "tomorrow", "book the room"])
eq("a task without one", M.captureArgs("tlog", "call Ada", false), ["tlog", "add", "-todo", "call Ada"])
eq("a note keeps its @", M.captureArgs("tlog", "met Ada @lunch", true), ["tlog", "add", "met Ada @lunch"])
eq("nothing typed writes nothing", M.captureArgs("tlog", "   ", false), null)

eq("plain reads as prose", M.plain("ask [[Elke Hofmann]] about [the brief](https://x.y) `now` **soon**"),
  "ask Elke Hofmann about the brief now soon")
eq("a block reference shows its page", M.plain("see [[Plan#^abc12]]"), "see Plan")
eq("today's journal is named as such", M.pageLabel({ page: "2026-11-03" }, "2026-11-03"), "today's journal")

if (failed) { console.error(`${failed} failed`); process.exit(1) }
console.log("omarchy widget: all pass")
