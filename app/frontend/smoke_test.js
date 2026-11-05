// Does the window actually come up?
//
// app_test.js checks the pure helpers — decorate, slashAt, shouldReload — and
// found real bugs. It could not find these, because nothing here had ever run
// the page: three separate crashes, all present since the first commit, all of
// which left the window drawing an empty pane.
//
//   - markActive read page.rel before the first page was open, so start() threw
//     and never opened today's journal
//   - markdown.Prop had no json tags, so the wire said Key/Value while this
//     file reads p.key, and any page with a block property threw while drawing
//   - the checkbox was drawn beside a task and the [ ] marker left in the text
//
// So this boots the real index.html and the real app.js against a stubbed Go
// API and asserts the page is there. jsdom is not vendored: without it the file
// says so and passes, because a missing test dependency is not a failing test.

let JSDOM;
try {
  ({ JSDOM } = require("jsdom"));
} catch {
  console.log("frontend smoke: skipped — jsdom is not on NODE_PATH (README, Development)");
  process.exit(0);
}

const fs = require("fs");
const path = require("path");
const here = __dirname;

// One page with everything that has ever broken drawing: a task, a deadline, a
// block property, a fence, a link, a tag, a block reference and an embed.
const PAGE = {
  rel: "journals/2026-09-25.md",
  title: "2026-09-25",
  hash: "abc123",
  isJournal: true,
  tags: [],
  blocks: [
    {
      addr: "journals/2026-09-25.md:0@abc123", offset: 0, depth: 0,
      text: "meeting with [[Ada Lovelace]] about [[analytical engine #project]]",
      body: "meeting with [[Ada Lovelace]] about [[analytical engine #project]]",
      hasChildren: true,
    },
    {
      addr: "journals/2026-09-25.md:70@abc123", offset: 70, depth: 1,
      text: "[ ] she will send the notes", body: "she will send the notes",
      task: "open", hasChildren: false,
      props: [{ key: "Deadline", value: "2026-09-26" }],
      due: "2026-09-26", dueState: "soon", dueLabel: "morgen",
    },
    {
      addr: "journals/2026-09-25.md:130@abc123", offset: 130, depth: 0,
      text: "a fence:\n```go\nfunc main() {}\n```", body: "a fence:\n```go\nfunc main() {}\n```",
      hasChildren: false,
      fences: [{ lang: "go", code: "func main() {}", tokens: [] }],
    },
    {
      addr: "journals/2026-09-25.md:200@abc123", offset: 200, depth: 0,
      text: "remember: [[Timetable#^k3f9q2]]", body: "remember: [[Timetable#^k3f9q2]]",
      hasChildren: false,
      embeds: [{ anchor: "k3f9q2", page: "Timetable", rel: "pages/Timetable.md",
                 addr: "pages/Timetable.md:12@def456", text: "the lecture is at nine" }],
    },
    {
      addr: "journals/2026-09-25.md:250@abc123", offset: 250, depth: 0,
      text: "[[Timetable#^k3f9q2]]", body: "[[Timetable#^k3f9q2]]",
      hasChildren: false, isEmbed: true,
      embeds: [{ anchor: "k3f9q2", page: "Timetable", rel: "pages/Timetable.md",
                 addr: "pages/Timetable.md:12@def456", text: "the lecture is at nine" }],
      embedKids: [{ anchor: "", page: "Timetable", rel: "pages/Timetable.md",
                    addr: "pages/Timetable.md:40@def456", text: "room B103", depth: 1 }],
    },
  ],
  tagged: [],
  refs: [
    { addr: "pages/Ada Lovelace.md:0@x", page: "Ada Lovelace", rel: "pages/Ada Lovelace.md",
      offset: 0, text: "she mentioned it", depth: 0, head: true },
    { addr: "pages/Ada Lovelace.md:30@x", page: "Ada Lovelace", rel: "pages/Ada Lovelace.md",
      offset: 30, text: "[ ] follow up", body: "follow up", task: "open", depth: 1 },
  ],
};

const OTHER = {
  rel: "pages/Timetable.md", title: "Timetable", hash: "def456",
  isJournal: false, tags: [], tagged: [], refs: [],
  blocks: [{ addr: "pages/Timetable.md:0@def456", offset: 0, depth: 0,
             text: "the lecture is at nine", body: "the lecture is at nine", hasChildren: false }],
};

// The journals page: today without a file, and yesterday with one block.
const TODAY = {
  rel: "journals/2026-11-02.md", title: "2026-11-02", hash: "", isJournal: true,
  tags: [], tagged: [], refs: [], blocks: [],
};
const YESTERDAY = {
  rel: "journals/2026-11-01.md", title: "2026-11-01", hash: "y1", isJournal: true,
  tags: [], tagged: [], refs: [],
  blocks: [{ addr: "journals/2026-11-01.md:0@y1", offset: 0, depth: 0,
             text: "yesterday's note", body: "yesterday's note", hasChildren: false }],
};
const DAYS = [
  { page: TODAY, date: "2026-11-02", weekday: "Monday", today: true, ahead: false },
  { page: YESTERDAY, date: "2026-11-01", weekday: "Sunday", today: false, ahead: false },
];
// Twenty days, so that a second batch is there to be opened.
const DAY_RELS = [TODAY.rel, YESTERDAY.rel];
for (let d = 31; DAY_RELS.length < 20; d--) DAY_RELS.push(`journals/2026-10-${d}.md`);

const opened = [];
const ran = []; // slash commands the window asked the core to run
const visited = [];
const dayCalls = [];
let recentList = [
  { rel: "pages/Timetable.md", title: "Timetable" },
  { rel: "journals/2026-09-25.md", title: "2026-09-25" },
  { rel: "journals", title: "Journals" },
];

const API = {
  Root: async () => "/home/ada/notes",
  Index: async () => ({ journals: ["2026-09-25"], pages: ["Ada Lovelace", "Timetable"], tags: ["project"] }),
  TodayJournal: async () => { opened.push("TodayJournal"); return PAGE; },
  Today: async () => { opened.push("Today"); return PAGE; },
  InsertBefore: async () => ({ page: PAGE, offset: 0 }),
  SetText: async () => ({ page: PAGE, offset: 0 }),
  DeleteBlock: async () => ({ page: PAGE, offset: 0 }),
  SplitBlock: async () => ({ page: PAGE, offset: 0 }),
  Indent: async () => ({ page: PAGE, offset: 0 }),
  Outdent: async () => ({ page: PAGE, offset: 0 }),
  Move: async () => ({ page: PAGE, offset: 0 }),
  MergeIntoPrevious: async () => ({ page: PAGE, offset: 0 }),
  OpenRel: async (rel) => (rel === "pages/Timetable.md" ? OTHER : PAGE),
  OpenPage: async () => OTHER,
  Journal: async () => PAGE,
  Search: async () => [{ addr: "a:0@b", rel: "a", page: "Ada Lovelace", offset: 0, hash: "b", text: "a hit" }],
  Due: async () => [{ addr: "a:0@b", rel: "a", page: "Ada Lovelace", offset: 0, hash: "b",
                      text: "follow up", due: "2026-09-26", state: "soon", label: "morgen" }],
  Sync: async () => ({ lastError: "" }),
  Settings: async () => [
    { key: "notes", value: "~/notes", kind: "text", hint: "where the notes live", source: "config" },
    { key: "git.autocommit", value: "true", kind: "bool", hint: "commit as you write", source: "config" },
  ],
  ConfigPath: async () => "~/.config/tlog/config.toml",
  // Only a bare slash finds anything, so no other test opens the menu.
  Commands: async (name) => (name === "" ? [{ name: "todo", title: "TODO", hint: "Task, open" }] : []),
  RunCommand: async (...args) => { ran.push(args[3]); return null; },
  CompletePages: async () => [],
  CompleteTags: async () => [],
  Blocks: async () => [],
  Places: async (q) => {
    const all = [
      { kind: "journal", name: "2026-09-25", rel: "journals/2026-09-25.md" },
      { kind: "page", name: "Ada Lovelace", rel: "pages/Ada Lovelace.md" },
      { kind: "page", name: "Timetable", rel: "pages/Timetable.md" },
      { kind: "tag", name: "project", rel: "" },
    ];
    return q ? all.filter((p) => p.name.toLowerCase().includes(q.toLowerCase())) : all;
  },
  AttachDir: async () => "~/.att",
  JournalsRel: async () => "journals",
  Recent: async () => ({ places: recentList }),
  // The core moves the place to the front; this does the same, so cycling can
  // be checked against a list that behaves like the real one.
  Visited: async (rel) => {
    visited.push(rel);
    const known = recentList.find((p) => p.rel === rel) || { rel, title: rel };
    recentList = [known, ...recentList.filter((p) => p.rel !== rel)];
    return { places: recentList };
  },
  JournalDays: async () => DAY_RELS,
  OpenDays: async (rels) => {
    dayCalls.push(rels);
    return rels.map((rel) => DAYS.find((d) => d.page.rel === rel) || {
      page: { rel, title: rel.slice(9, 19), hash: "h" + rel, isJournal: true,
              tags: [], tagged: [], refs: [], blocks: [] },
      date: rel.slice(9, 19), weekday: "Someday", today: false, ahead: false,
    });
  },
  Attachments: async () => [],
  OpenURL: async (rel, url) => {
    opened.push("OpenURL " + rel + " " + url);
    return url.endsWith(".md") ? { name: "Regeln.md", path: "/home/ada/notes/assets/Regeln.md",
                                   lang: "markdown", text: "# Regeln\nNote < 4" } : null;
  },
  OpenOutside: async (p) => { opened.push("OpenOutside " + p); },
};

let failures = 0;
function ok(label, cond, detail) {
  if (!cond) {
    console.log(`FAIL  ${label}${detail ? "\n  " + detail : ""}`);
    failures++;
  }
}

(async () => {
  const dom = new JSDOM(fs.readFileSync(path.join(here, "index.html"), "utf8"), {
    runScripts: "outside-only",
    pretendToBeVisual: true,
    url: "http://localhost/",
  });
  // The stylesheet is linked, not inlined, and jsdom does not fetch it. It has
  // to be here: whether an element is actually hidden is a question about the
  // cascade, and [hidden] loses to any id selector that sets display.
  const style = dom.window.document.createElement("style");
  style.textContent = fs.readFileSync(path.join(here, "style.css"), "utf8");
  dom.window.document.head.appendChild(style);

  // jsdom has no innerText, and raw() is built on it — so without this shim
  // every editing path is silently unreachable here: splitting, indenting,
  // merging and saving on blur all start by reading the block's text and would
  // throw on undefined before doing anything. A contenteditable holds one text
  // node and <br> for the line breaks, which is what this reproduces.
  Object.defineProperty(dom.window.HTMLElement.prototype, "innerText", {
    configurable: true,
    get() {
      return [...this.childNodes]
        .map((n) => (n.nodeName === "BR" ? "\n" : n.textContent))
        .join("");
    },
    set(v) {
      this.textContent = v;
    },
  });

  // Nor does it have contentEditable as a property: setting it is a plain
  // expando, the attribute never appears, and a block cannot take focus — so
  // anything that moves the caret from block to block could not be checked.
  Object.defineProperty(dom.window.HTMLElement.prototype, "contentEditable", {
    configurable: true,
    get() {
      return this.getAttribute("contenteditable") || "inherit";
    },
    set(v) {
      this.setAttribute("contenteditable", v);
    },
  });
  // And no layout: a range has no box, so a pop-up placed at the caret (the
  // slash menu) has nowhere to go. A zero box puts it in the corner.
  dom.window.Range.prototype.getBoundingClientRect = () =>
    ({ left: 0, top: 0, right: 0, bottom: 0, width: 0, height: 0, x: 0, y: 0 });

  const w = dom.window;
  w.go = { main: { API } };
  w.runtime = undefined;

  const errors = [];
  w.addEventListener("error", (e) => errors.push(e.error ? e.error.stack : e.message));
  w.addEventListener("unhandledrejection", (e) => errors.push(e.reason && e.reason.stack));
  // An exception in start() surfaces as a rejected promise, which by default
  // takes the whole process down before a single assertion has run — the
  // failure would be real but the report would be a stack trace and nothing
  // else. Recording it instead is what lets this file say which part broke.
  process.on("unhandledRejection", (r) => errors.push("unhandled: " + (r && r.stack ? r.stack : r)));
  process.on("uncaughtException", (e) => errors.push("uncaught: " + e.stack));

  try {
    w.eval(fs.readFileSync(path.join(here, "app.js"), "utf8"));
  } catch (e) {
    errors.push("threw while loading: " + e.stack);
  }
  await new Promise((r) => setTimeout(r, 300));

  const $ = (id) => w.document.getElementById(id);
  ok("the page comes up without throwing", errors.length === 0, errors.join("\n  "));

  // Not "the attribute is set" — whether it is actually off the screen.
  const shown = (id) => w.getComputedStyle($(id)).display !== "none";
  ok("the settings sheet is not covering the page", !shown("settings"));
  ok("the completion popup is not floating over the page", !shown("complete"));
  ok("the title is drawn", $("title").textContent === "2026-09-25", $("title").textContent);
  ok("the outline is drawn", $("outline").children.length > 0);
  ok("the sidebar is filled", $("pages").children.length === 2);

  // `startup = last` is a setting in all three menus. The window called
  // TodayJournal at launch, which always opens today, so the setting did
  // nothing here while the outliner honoured it.
  ok("the window opens where the startup setting says",
    opened[0] === "Today", JSON.stringify(opened));

  const outline = $("outline").textContent;
  ok("a task does not show its marker twice", !outline.includes("[ ]"), outline.slice(0, 200));
  ok("a task shows a checkbox", $("outline").querySelector(".check") !== null);
  ok("a block property is drawn", $("outline").querySelector(".props") === null ||
    $("outline").querySelector(".props").textContent.includes("Deadline"));
  ok("a deadline chip is drawn", $("outline").querySelector(".due") !== null);
  ok("a fence is highlighted", $("outline").querySelector("pre.code") !== null);
  ok("a link is drawn as its name", outline.includes("Ada Lovelace"));

  ok("a reference draws the block it points at", outline.includes("the lecture is at nine"),
    "got: " + outline.slice(-160));
  ok("a reference does not leak its anchor", !outline.includes("#^"));
  ok("an embed brings its subtree", outline.includes("room B103"));
  ok("embedded rows are marked as such", $("outline").querySelector(".block.embedded") !== null);

  const refs = $("reflist").textContent;
  ok("backlinks are drawn", refs.includes("she mentioned it"));
  ok("a task in a backlink does not show its marker", !refs.includes("[ ]"), refs);

  await w.eval("openSettings()");
  await new Promise((r) => setTimeout(r, 100));
  ok("settings open", $("settings").hidden === false);
  ok("settings are listed", $("settingslist").querySelectorAll(".setting").length === 2);
  ok("a text setting carries its value",
    $("settingslist").querySelector("input[type=text]").value === "~/notes");

  $("settings").hidden = true;

  await w.eval("openAgenda()");
  await new Promise((r) => setTimeout(r, 100));
  ok("the agenda opens", $("agenda").hidden === false);
  ok("the agenda lists what is due", $("agendalist").textContent.includes("follow up"));
  ok("over the page, not instead of it", $("outline").hidden === false);
  await w.eval("closeAgenda()");

  // --- going back ---------------------------------------------------------
  //
  // Following a link used to be a one-way door: the only route back was
  // finding the page again in the sidebar.
  await w.eval(`openRel("pages/Timetable.md")`);
  await new Promise((r) => setTimeout(r, 100));
  ok("following a link arrives", $("title").textContent === "Timetable");
  ok("back is offered once there is somewhere to go", $("back").disabled === false);

  await w.eval("goBack()");
  await new Promise((r) => setTimeout(r, 100));
  ok("back returns to where you were", $("title").textContent === "2026-09-25",
    $("title").textContent);
  ok("forward is offered after going back", $("fwd").disabled === false);
  ok("back is not offered at the start", $("back").disabled === true);

  await w.eval("goForward()");
  await new Promise((r) => setTimeout(r, 100));
  ok("forward lands where back came from", $("title").textContent === "Timetable");

  // Arriving at the page you are already on is not a move.
  const depth = await w.eval("history.length");
  await w.eval(`openRel("pages/Timetable.md")`);
  await new Promise((r) => setTimeout(r, 100));
  ok("re-opening the current page does not stack up history",
    (await w.eval("history.length")) === depth);

  // --- folding ------------------------------------------------------------
  //
  // The window could not fold anything: hasChildren changed a bullet's colour
  // and nothing else, so a long journal had to be read in full. Driven by
  // clicking the bullet, which is how anyone else would do it.
  await w.eval(`openRel("journals/2026-09-25.md")`);
  await new Promise((r) => setTimeout(r, 100));
  const rows = () => $("outline").querySelectorAll(".block").length;
  const parentBullet = () => $("outline").querySelector(".block .bullet.haskids");
  const before = rows();
  ok("the child is on screen to begin with", $("outline").textContent.includes("she will send"));
  ok("a parent has a foldable bullet", parentBullet() !== null);

  parentBullet().click();
  await new Promise((r) => setTimeout(r, 50));
  ok("folding hides the subtree", rows() < before, `${rows()} of ${before}`);
  ok("and the child is gone", !$("outline").textContent.includes("she will send"));
  ok("the parent is still there", $("outline").textContent.includes("meeting with"));
  ok("the bullet says it is folded", $("outline").querySelector(".bullet.folded") !== null);

  parentBullet().click();
  await new Promise((r) => setTimeout(r, 50));
  ok("unfolding brings it back", rows() === before);
  ok("and the child is back", $("outline").textContent.includes("she will send"));

  // A bullet folds and does nothing else. It used to make the block a task,
  // so a stray click rewrote a note into a checkbox.
  const plain = [...$("outline").querySelectorAll(".block .bullet")]
    .find((el) => !el.classList.contains("haskids"));
  ok("a childless block has an inert bullet", plain && plain.onclick === null);
  let toggled = false;
  const realToggle = API.ToggleTask;
  API.ToggleTask = async () => { toggled = true; return { page: PAGE, offset: 0 }; };
  if (plain) plain.click();
  parentBullet().click();
  await new Promise((r) => setTimeout(r, 50));
  ok("clicking a bullet never makes the block a task", toggled === false);
  parentBullet().click();
  await new Promise((r) => setTimeout(r, 50));

  // The checkbox still ticks off — that is what a checkbox is for.
  $("outline").querySelector(".check").click();
  await new Promise((r) => setTimeout(r, 50));
  ok("a checkbox still ticks off", toggled === true);
  API.ToggleTask = realToggle;

  // A fold belongs to the block, not to the row it happened to be on.
  parentBullet().click();
  await new Promise((r) => setTimeout(r, 50));
  await w.eval(`openRel("pages/Timetable.md")`);
  await new Promise((r) => setTimeout(r, 100));
  ok("a fold does not follow you to another page",
    $("outline").querySelector(".bullet.folded") === null);
  await w.eval("goBack()");
  await new Promise((r) => setTimeout(r, 100));
  ok("and is still there when you come back",
    $("outline").querySelector(".bullet.folded") !== null);
  parentBullet().click();
  await new Promise((r) => setTimeout(r, 50));

  // --- deleting, and inserting above ---------------------------------------
  //
  // The window could only delete an empty first block, so a paragraph could
  // not be thrown away at all; and it had no way to make a block above one.
  {
    let deleted = null, inserted = false;
    const realDelete = API.DeleteBlock, realInsert = API.InsertBefore;
    API.DeleteBlock = async (rel, offset) => { deleted = offset; return { page: PAGE, offset: 0 }; };
    API.InsertBefore = async () => { inserted = true; return { page: PAGE, offset: 0 }; };

    const block = $("outline").querySelector(".text");
    const key = (init) => block.dispatchEvent(new w.KeyboardEvent("keydown",
      { bubbles: true, cancelable: true, ...init }));

    key({ key: "Backspace", metaKey: true });
    await new Promise((r) => setTimeout(r, 60));
    ok("cmd-backspace deletes the block", deleted === 0, String(deleted));
    ok("and says git has it", $("error").textContent.includes("git has the previous version"),
      $("error").textContent);
    ok("it counts what goes with it", $("error").textContent.includes("child"),
      $("error").textContent);

    key({ key: "Enter", metaKey: true, shiftKey: true });
    await new Promise((r) => setTimeout(r, 60));
    ok("cmd-shift-enter makes a block above", inserted === true);

    API.DeleteBlock = realDelete;
    API.InsertBefore = realInsert;
  }

  // --- going somewhere ------------------------------------------------------
  //
  // Search finds words inside blocks, which is the wrong tool for "take me to
  // the page about Ada" — a page whose name you remember but whose contents
  // you do not never came up.
  await w.eval("openPalette()");
  await new Promise((r) => setTimeout(r, 100));
  ok("the palette opens", $("palette").hidden === false);
  ok("it offers everywhere you could go, the journals page first",
    $("palettelist").children.length === 5 &&
    $("palettelist").children[0].textContent.includes("Journals"),
    String($("palettelist").children.length));
  ok("it says which kind each one is", $("palettelist").textContent.includes("journal") &&
    $("palettelist").textContent.includes("tag"));
  ok("a tag is shown as one", $("palettelist").textContent.includes("#project"));

  $("palettequery").value = "table";
  await w.eval("refreshPalette()");
  await new Promise((r) => setTimeout(r, 80));
  ok("typing narrows it", $("palettelist").children.length === 1,
    $("palettelist").textContent);

  await w.eval("choosePlace()");
  await new Promise((r) => setTimeout(r, 100));
  ok("choosing goes there", $("title").textContent === "Timetable", $("title").textContent);
  ok("and the palette closes", $("palette").hidden === true);

  // --- the keyboard -------------------------------------------------------
  await w.eval("toggleHelp()");
  ok("the help page opens", $("help").hidden === false);
  ok("it lists the bindings from the table",
    $("helplist").textContent.includes("search") &&
    $("helplist").textContent.includes("back") &&
    $("helplist").textContent.includes("agenda"));
  ok("and says what each part of the window does, as the outliner's help does",
    ["Writing", "Selecting blocks", "Going places", "Mentioning", "Tasks and dates",
     "Links and files", "The mouse", "Commands"].every((h) =>
      [...$("helplist").querySelectorAll("h3")].some((el) => el.textContent === h)),
    [...$("helplist").querySelectorAll("h3")].map((el) => el.textContent).join(" · "));
  ok("with the notes directory named", $("helplist").textContent.includes("/home/ada/notes"));
  await w.eval("toggleHelp()");
  ok("and closes again", $("help").hidden === true);
  {
    const press = (init) => w.document.dispatchEvent(new w.KeyboardEvent("keydown",
      { bubbles: true, cancelable: true, ...init }));
    press({ key: "h", ctrlKey: true });
    ok("ctrl+h opens it", $("help").hidden === false);
    press({ key: "Escape" });
    ok("esc closes it", $("help").hidden === true);
    press({ key: "h", ctrlKey: true });
    press({ key: "h", ctrlKey: true });
    ok("and ctrl+h again does too", $("help").hidden === true);
    press({ key: "h", metaKey: true });
    ok("⌘H is left to the system", $("help").hidden === true);

    // Opened while writing, it takes the keys: nothing types into, or moves
    // the caret in, a block hidden behind it.
    const block = $("outline").querySelector(".block > .text[data-offset]");
    w.__el = block;
    w.eval("placeCaretAt(__el, 2)");
    const send = (init) => {
      const ev = new w.KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init });
      block.dispatchEvent(ev);
      return ev;
    };
    send({ key: "h", ctrlKey: true });
    ok("ctrl+h opens it from inside a block", $("help").hidden === false);
    const down = send({ key: "ArrowDown" });
    const typed = send({ key: "q" });
    const enter = send({ key: "Enter" });
    ok("the keys are the help's, not the block's",
      down.defaultPrevented && typed.defaultPrevented && enter.defaultPrevented &&
      $("help").hidden === false && w.eval("caretIndex(document.activeElement)") === 2);
    send({ key: "Escape" });
    ok("esc closes it and the caret is where it was", $("help").hidden === true &&
      w.document.activeElement === block && w.eval("caretIndex(document.activeElement)") === 2);
  }

  // --- a slash typed as a character -----------------------------------------
  //
  // A bare slash shows the menu, but enter after it used to run the first row
  // — /todo — and turn "a /" into a task. Nothing was asked for, so the key is
  // the block's; a row chosen with the arrows is still a choice.
  {
    const block = $("outline").querySelector(".block > .text[data-offset]");
    const before = block.textContent;
    const send = (init) => {
      const ev = new w.KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init });
      block.dispatchEvent(ev);
      return ev;
    };
    const tick = () => new Promise((r) => setTimeout(r, 20));
    block.focus();
    block.textContent = "a /";
    w.__el = block;
    w.eval("placeCaretAt(__el, 3)");
    await w.eval("updateCompletion(__el)");
    ok("a bare slash shows the menu", $("complete").hidden === false);
    send({ key: "Enter" }); // the block's own enter: it splits the block
    await tick();
    ok("enter after a bare slash is the block's, not the menu's",
      ran.length === 0 && $("complete").hidden === true, JSON.stringify(ran));

    block.textContent = "a /";
    w.eval("placeCaretAt(__el, 3)");
    await w.eval("updateCompletion(__el)");
    send({ key: "ArrowDown" });
    await tick();
    send({ key: "Enter" });
    await tick();
    ok("a row chosen with the arrows still runs on enter", ran[0] === "todo", JSON.stringify(ran));
    w.eval("hideCompletion()");
    block.textContent = before;
  }

  // --- what the window says ------------------------------------------------
  //
  // An error used to appear for six seconds and then be gone with no way to
  // get it back. If you were looking at the other screen when the push failed,
  // it never happened.
  $("help").hidden = true;
  $("log").hidden = true;

  await w.eval(`say("a quiet receipt")`);
  ok("a note is shown", $("error").hidden === false);
  ok("a note is not styled as a failure", !$("error").classList.contains("bad"));

  // A note removes itself, and the timer it sets is the evidence.
  ok("a note schedules its own disappearance", (await w.eval("!!say.timer")) === true);

  await w.eval("clearTimeout(say.timer); say.timer = undefined");
  await w.eval(`fail(new Error("push rejected"))`);
  ok("an error is shown", $("error").hidden === false);
  ok("an error is styled as one", $("error").classList.contains("bad"));
  ok("an error says what happened", $("error").textContent.includes("push rejected"));
  ok("an error does not schedule its own disappearance",
    (await w.eval("say.timer")) === undefined);

  $("error").click();
  await new Promise((r) => setTimeout(r, 50));
  ok("dismissing an error opens the log rather than losing it", $("log").hidden === false);
  ok("the log has both lines", $("loglist").textContent.includes("push rejected") &&
    $("loglist").textContent.includes("a quiet receipt"));
  ok("the log is newest first",
    $("loglist").textContent.indexOf("push rejected") <
      $("loglist").textContent.indexOf("a quiet receipt"));
  $("log").hidden = true;

  // --- the recent list -------------------------------------------------------
  //
  // The header names the places you were last, from the same list the
  // outliner keeps, and each arrival is told to the core.
  ok("arriving somewhere is remembered", visited.includes("pages/Timetable.md"),
    JSON.stringify(visited));
  const strip = () => [...$("recent").querySelectorAll(".place")].map((el) => el.textContent);
  ok("the header shows the recent places", strip().includes("Journals"), strip().join(" · "));
  ok("the place you are on is marked",
    $("recent").querySelector(".place.current").textContent === $("title").textContent,
    $("recent").innerHTML);

  // ctrl+n steps through them and the order holds still while it does.
  const key = (init) => w.document.dispatchEvent(new w.KeyboardEvent("keydown",
    { bubbles: true, cancelable: true, ...init }));
  const startAt = $("title").textContent;
  const order = strip();
  const before2 = visited.length;
  key({ key: "n", ctrlKey: true });
  await new Promise((r) => setTimeout(r, 100));
  ok("ctrl+n goes to the place you were before", $("title").textContent === order[1],
    `${$("title").textContent} after ${order.join(" · ")}`);
  key({ key: "n", ctrlKey: true });
  await new Promise((r) => setTimeout(r, 150));
  ok("and again to the one before that", $("title").textContent === order[2],
    `${$("title").textContent} after ${order.join(" · ")}`);
  ok("the order holds still while stepping", strip().join("|") === order.join("|"),
    strip().join(" · "));
  ok("nothing is remembered while stepping", visited.length === before2);
  key({ key: "x" });
  await new Promise((r) => setTimeout(r, 100));
  ok("another key settles it where it stopped", visited[visited.length - 1] !== undefined &&
    strip()[0] === $("title").textContent, strip().join(" · "));
  ok("the place stopped on is now first", startAt !== strip()[0]);

  // The agenda opens over the page: a glance at what is due, after which you
  // are where you were. It is not in the header and not a recent place.
  {
    const settle = () => new Promise((r) => setTimeout(r, 100));
    const press = (init, target = w.document.activeElement || w.document) => {
      const ev = new w.KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init });
      target.dispatchEvent(ev);
      return ev;
    };
    const here = $("title").textContent;
    const order = strip().join("|");
    ok("the header holds only recent places", !strip().includes("Agenda"), strip().join(" · "));
    press({ key: "a", metaKey: true, shiftKey: true }, w.document);
    await settle();
    ok("⌘⇧A opens the agenda over the page", $("agenda").hidden === false &&
      $("outline").hidden === false && $("title").textContent === here);
    ok("it is not added to the recent places", strip().join("|") === order, strip().join(" · "));
    press({ key: "Escape" }, w.document);
    ok("esc closes it", $("agenda").hidden === true);

    // Opened while writing, it takes the keys and leaves the caret alone.
    const block = $("outline").querySelector(".block > .text[data-offset]");
    w.__el = block;
    w.eval("placeCaretAt(__el, 3)");
    press({ key: "a", metaKey: true, shiftKey: true });
    await settle();
    ok("⌘⇧A opens it from inside a block", $("agenda").hidden === false);
    const down = press({ key: "ArrowDown" });
    const typed = press({ key: "q" });
    ok("the arrows and letters are the agenda's, not the block's",
      down.defaultPrevented && typed.defaultPrevented && w.document.activeElement === block &&
      w.eval("caretIndex(document.activeElement)") === 3);
    press({ key: "a", metaKey: true, shiftKey: true });
    ok("⌘⇧A again closes it", $("agenda").hidden === true);
    ok("and the caret is where it was", w.document.activeElement === block &&
      w.eval("caretIndex(document.activeElement)") === 3);

    // Enter goes to the chosen block.
    const asked = [];
    const realOpen = API.OpenRel;
    API.OpenRel = async (rel) => { asked.push(rel); return OTHER; };
    w.eval("openAgenda()");
    await settle();
    ok("the first line is chosen", $("agendalist").querySelector(".agendaitem.sel") !== null);
    press({ key: "Enter" }, w.document);
    await settle();
    ok("enter goes to the block's page", asked.join() === "a" && $("title").textContent === "Timetable" &&
      $("agenda").hidden === true, `${asked.join()} ${$("title").textContent}`);
    API.OpenRel = realOpen;
    await w.eval("goBack()");
    await settle();
    ok("and back returns from it", $("title").textContent === here, $("title").textContent);

    // Ticking a line off in the pop-up, as a checkbox on a page does: the
    // agenda stays open, the list is asked for again, and the page behind is
    // redrawn with the caret where it was.
    w.__page = PAGE;
    w.eval("show(__page)");
    await settle();
    let open = [
      { addr: PAGE.rel + ":70@abc123", rel: PAGE.rel, page: "2026-09-25", offset: 70, hash: "abc123",
        text: "she will send the notes", due: "2026-09-26", state: "soon", label: "morgen", done: false },
      { addr: PAGE.rel + ":200@abc123", rel: PAGE.rel, page: "2026-09-25", offset: 200, hash: "abc123",
        text: "remember", due: "2026-09-27", state: "soon", label: "übermorgen", done: false },
    ];
    let closed = [];
    const dueAsked = [], commands = [], saves = [], opened2 = [];
    const was = { Due: API.Due, ApplyCommand: API.ApplyCommand, SetText: API.SetText, OpenRel: API.OpenRel };
    API.Due = async (all) => { dueAsked.push(all); return all ? [...open, ...closed] : open; };
    API.ApplyCommand = async (rel, offset, hash, name) => {
      commands.push([rel, offset, hash, name]);
      const it = [...open, ...closed].find((x) => x.offset === offset);
      if (name === "done") { open = open.filter((x) => x !== it); closed = [...closed, { ...it, done: true, state: "" }]; }
      else { closed = closed.filter((x) => x !== it); open = [...open, { ...it, done: false }]; }
      return { page: { ...PAGE, hash: "after" + commands.length }, offset };
    };
    API.SetText = async (rel, offset, hash, text) => { saves.push([offset, text]); return { page: PAGE, offset }; };
    API.OpenRel = async (rel) => { opened2.push(rel); return PAGE; };

    // Writing in a block, with something typed that is not saved yet.
    const writing = () => $("outline").querySelector('.block > .text[data-offset="0"]');
    w.__el = writing();
    w.eval("placeCaretAt(__el, null)");
    writing().textContent = "meeting, typed";
    w.__el = writing();
    w.eval("placeCaretAt(__el, 4)");
    press({ key: "a", metaKey: true, shiftKey: true }, writing());
    await settle();
    ok("opening the agenda saves what was typed first", JSON.stringify(saves) === JSON.stringify([[0, "meeting, typed"]]),
      JSON.stringify(saves));
    ok("and the caret is still in its block", Number(w.document.activeElement.dataset.offset) === 0 &&
      w.eval("caretIndex(document.activeElement)") === 4);
    const checks = () => [...$("agendalist").querySelectorAll(".agendaitem .check")];
    ok("each line has a box to tick", checks().length === 2 && checks()[0].textContent === "☐",
      $("agendalist").innerHTML);

    checks()[0].click();
    await settle();
    ok("clicking the box closes the line, by its address",
      JSON.stringify(commands[0]) === JSON.stringify([PAGE.rel, 70, "abc123", "done"]), JSON.stringify(commands));
    ok("without going anywhere", opened2.length === 0 && $("agenda").hidden === false);
    ok("the list is asked for again, and the line is gone", checks().length === 1 &&
      !$("agendalist").textContent.includes("she will send"), $("agendalist").textContent);
    ok("the page behind keeps the caret where it was",
      Number(w.document.activeElement.dataset.offset) === 0 && w.eval("caretIndex(document.activeElement)") === 4,
      String(w.document.activeElement.dataset && w.document.activeElement.dataset.offset));

    press({ key: " " }, w.document);
    await settle();
    ok("space ticks off the chosen line", commands[1] && commands[1][1] === 200 && commands[1][3] === "done",
      JSON.stringify(commands));
    ok("and what it takes is the fresh list", $("agendalist").textContent.includes("nothing is due"),
      $("agendalist").textContent);

    press({ key: "a" }, w.document);
    await settle();
    ok("a shows what is done too", dueAsked[dueAsked.length - 1] === true && checks().length === 2 &&
      checks().every((c) => c.textContent === "☑") && $("agendatitle").textContent.includes("everything"));
    checks()[0].click();
    await settle();
    ok("and ☑ opens it again", commands[2] && commands[2][3] === "todo", JSON.stringify(commands));
    ok("still in the agenda", $("agenda").hidden === false && saves.length === 1);

    press({ key: "Escape" }, w.document);
    Object.assign(API, was);
  }

  // --- the journals page ----------------------------------------------------
  await w.eval("openJournals()");
  await new Promise((r) => setTimeout(r, 150));
  ok("the journals page opens", $("title").textContent === "Journals", $("title").textContent);
  const sections = () => $("outline").querySelectorAll("section.day");
  ok("every day is drawn under its own date", sections().length === 14,
    String(sections().length));
  ok("newest first, today at the top",
    sections()[0].querySelector(".date").textContent === "2026-11-02" &&
    sections()[0].querySelector(".note").textContent.includes("today"));
  ok("today without a file can be started",
    sections()[0].textContent.includes("click here to start writing"));
  ok("a day's blocks are there", sections()[1].textContent.includes("yesterday's note"));
  ok("the journals page is in the recent list", visited[visited.length - 1] === "journals");
  ok("back is offered from the journals page", $("back").disabled === false);

  // An edit is addressed to the day it was made on, with that day's hash.
  {
    let call = null;
    const real = API.SetText;
    API.SetText = async (rel, offset, hash, text) => {
      call = { rel, offset, hash, text };
      return { page: { ...YESTERDAY, hash: "y2",
        blocks: [{ ...YESTERDAY.blocks[0], text, body: text }] }, offset };
    };
    const el = sections()[1].querySelector(".text");
    el.dispatchEvent(new w.FocusEvent("focus"));
    el.textContent = "yesterday, edited";
    el.dispatchEvent(new w.FocusEvent("blur"));
    await new Promise((r) => setTimeout(r, 100));
    ok("an edit goes to its own day", call && call.rel === YESTERDAY.rel && call.hash === "y1",
      JSON.stringify(call));
    ok("only that day is redrawn, with what was written",
      sections()[1].textContent.includes("yesterday, edited") && sections().length === 14);
    ok("and the next edit carries the new hash",
      (await w.eval(`viewOf("${YESTERDAY.rel}").hash`)) === "y2");
    API.SetText = real;
  }

  // Older days are opened as they are asked for, not all at once.
  ok("only the first fortnight is opened", dayCalls[dayCalls.length - 1].length === 14);
  $("outline").querySelector(".moredays").click();
  await new Promise((r) => setTimeout(r, 100));
  ok("the rest are opened below", sections().length === 20, String(sections().length));
  ok("and nothing more is offered", $("outline").querySelector(".moredays") === null);

  // A date opens that day on its own.
  sections()[1].querySelector(".date").click();
  await new Promise((r) => setTimeout(r, 100));
  // The stub answers every day with the same page; what matters is that the
  // journals page was left for a single one.
  ok("a date opens its day", $("title").textContent === "2026-09-25" &&
    $("outline").querySelector("section.day") === null, $("title").textContent);
  await w.eval("goBack()");
  await new Promise((r) => setTimeout(r, 150));
  ok("back returns to the journals page", $("title").textContent === "Journals",
    $("title").textContent);

  // A link to a text file opens it over the page; esc puts it away again.
  // From today's page, which is the note the link below is written in.
  await w.eval("openToday()");
  await new Promise((r) => setTimeout(r, 150));
  ok("the file sheet is not covering the page", !shown("file"));
  const a = w.document.createElement("span");
  a.dataset.url = "../assets/Regeln.md";
  w.document.body.appendChild(a);
  a.dispatchEvent(new w.MouseEvent("mousedown", { bubbles: true }));
  await new Promise((r) => setTimeout(r, 50));
  ok("following a link asks with the note it is written in",
    opened.some((o) => /^OpenURL (journals|pages)\/\S.*\.md \.\.\/assets\/Regeln\.md$/.test(o)), opened.join(", "));
  ok("a text file is shown", shown("file") && $("filetext").textContent.includes("Note < 4"));
  ok("its headings stand out", $("filetext").querySelector("strong") !== null);
  $("fileoutside").click();
  await new Promise((r) => setTimeout(r, 50));
  ok("open outside hands it over", opened.includes("OpenOutside /home/ada/notes/assets/Regeln.md"));
  w.document.dispatchEvent(new w.KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  ok("esc puts it away", !shown("file"));

  // --- selecting blocks ------------------------------------------------------
  //
  // esc in a block selects it; ⌘C, ⌘X, ⌘V and Tab act on the selection; what
  // goes on the clipboard is what the core says the blocks are.
  {
    let clip = "";
    w.runtime = { ClipboardSetText: async (t) => { clip = t; return true; },
                  ClipboardGetText: async () => clip };
    const calls = [];
    const was = { CopyBlocks: API.CopyBlocks, CutBlocks: API.CutBlocks,
                  PasteBlocks: API.PasteBlocks, ShiftBlocks: API.ShiftBlocks };
    API.CopyBlocks = async (rel, hash, offs) => { calls.push(["copy", rel, offs]); return "- a\n- b\n"; };
    API.CutBlocks = async (rel, hash, offs) => { calls.push(["cut", rel, offs]);
      return { edit: { page: PAGE, offset: 0 }, text: "- cut\n" }; };
    API.PasteBlocks = async (rel, off, hash, text, before) => { calls.push(["paste", rel, off, text]);
      return { page: PAGE, offset: off }; };
    API.ShiftBlocks = async (rel, hash, offs, outdent) => { calls.push(["shift", rel, offs, outdent]);
      return { page: PAGE, offset: offs[0] }; };
    const key = (init, target = w.document) => target.dispatchEvent(new w.KeyboardEvent("keydown",
      { bubbles: true, cancelable: true, ...init }));
    const settle = () => new Promise((r) => setTimeout(r, 80));
    const texts = () => [...$("outline").querySelectorAll(".block > .text[data-offset]")];
    const selected = () => [...$("outline").querySelectorAll(".block.selected > .text")]
      .map((t) => Number(t.dataset.offset));

    const first = texts()[0];
    first.focus();
    key({ key: "Escape" }, first);
    await settle();
    ok("esc in a block selects it", selected().join() === "0", selected().join());
    key({ key: "ArrowDown", shiftKey: true });
    ok("shift+↓ grows the selection", selected().join() === "0,70", selected().join());

    key({ key: "c", metaKey: true });
    await settle();
    ok("⌘C copies the selected blocks", JSON.stringify(calls[0]) ===
      JSON.stringify(["copy", PAGE.rel, [0, 70]]), JSON.stringify(calls));
    ok("onto the system clipboard", clip === "- a\n- b\n", JSON.stringify(clip));

    key({ key: "Tab" });
    await settle();
    ok("Tab indents the selection together",
      JSON.stringify(calls[1]) === JSON.stringify(["shift", PAGE.rel, [0, 70], false]), JSON.stringify(calls));
    ok("and it stays selected", selected().length === 2, selected().join());

    key({ key: "x", metaKey: true });
    await settle();
    ok("⌘X cuts", calls[2] && calls[2][0] === "cut" && clip === "- cut\n", JSON.stringify(calls));
    ok("which ends the selection", selected().length === 0);

    key({ key: "Escape" }, texts()[2]);
    await settle();
    key({ key: "v", metaKey: true });
    await settle();
    ok("⌘V pastes below the selection", JSON.stringify(calls[3]) ===
      JSON.stringify(["paste", PAGE.rel, Number(texts()[2].dataset.offset), "- cut\n"]), JSON.stringify(calls));

    // An outline pasted into a block's text goes in as blocks.
    const t = texts()[0];
    t.focus();
    const ev = new w.Event("paste", { bubbles: true, cancelable: true });
    ev.clipboardData = { getData: () => "- one\n  - two", items: [] };
    t.dispatchEvent(ev);
    await settle();
    ok("an outline pasted into text becomes blocks", calls.some((c) => c[0] === "paste" && c[3] === "- one\n  - two"),
      JSON.stringify(calls));
    ok("and is not typed in as text", ev.defaultPrevented);

    key({ key: "Escape" });
    Object.assign(API, was);
    w.runtime = undefined;
  }

  // --- the arrows while writing ---------------------------------------------
  //
  // Each block is its own editable, so ↑ on the first line and ↓ on the last
  // had nowhere to go and the caret stayed where it was: the only way to the
  // next block was the mouse.
  {
    w.__page = PAGE;
    w.eval("show(__page)");
    await new Promise((r) => setTimeout(r, 60));
    const saves = [];
    const realSet = API.SetText;
    API.SetText = async (rel, offset, hash, text) => { saves.push([offset, text]); return { page: PAGE, offset }; };
    const texts = () => [...$("outline").querySelectorAll(".block > .text[data-offset]")];
    const at = () => Number(w.document.activeElement.dataset.offset);
    const settle = () => new Promise((r) => setTimeout(r, 60));
    const caret = (el, n) => { w.__el = el; w.eval(`placeCaretAt(__el, ${n === null ? "null" : n})`); };
    const arrow = (k) => {
      const ev = new w.KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true });
      w.document.activeElement.dispatchEvent(ev);
      return ev;
    };

    caret(texts()[0], null);
    let ev = arrow("ArrowDown");
    await settle();
    ok("↓ on a block's last line goes to the block below", ev.defaultPrevented && at() === 70, String(at()));
    ok("and the caret starts at its beginning", w.eval("caretIndex(document.activeElement)") === 0);

    ev = arrow("ArrowUp");
    await settle();
    ok("↑ on a block's first line goes to the block above", ev.defaultPrevented && at() === 0, String(at()));
    ok("and the caret lands at its end",
      w.eval("caretIndex(document.activeElement)") === PAGE.blocks[0].text.length);
    ok("moving through unchanged blocks writes nothing", saves.length === 0, JSON.stringify(saves));

    // A block of several lines: the arrows walk its lines before leaving it.
    const fence = texts().find((t) => Number(t.dataset.offset) === 130);
    caret(fence, 0);
    ev = arrow("ArrowDown");
    ok("↓ above a block's last line stays in it", !ev.defaultPrevented && at() === 130, String(at()));
    caret(fence, fence.textContent.length);
    ev = arrow("ArrowUp");
    ok("↑ below its first line stays in it", !ev.defaultPrevented && at() === 130, String(at()));

    caret(texts()[0], 0);
    ev = arrow("ArrowUp");
    ok("↑ in the first block is the browser's own", !ev.defaultPrevented && at() === 0, String(at()));

    // What was typed is saved on the way out, once, and the move still happens.
    const first = texts()[0];
    caret(first, null);
    first.textContent = "meeting, edited";
    caret(first, null);
    arrow("ArrowDown");
    await settle();
    ok("leaving a changed block by ↓ saves it", JSON.stringify(saves) === JSON.stringify([[0, "meeting, edited"]]),
      JSON.stringify(saves));
    ok("and arrives in the block below", at() === 70, String(at()));
    w.document.activeElement.blur();
    await settle();
    ok("the blur after it writes nothing more", saves.length === 1, JSON.stringify(saves));

    API.SetText = realSet;
  }

  console.log(failures === 0 ? "frontend smoke: all pass" : `frontend smoke: ${failures} FAILURES`);
  process.exit(failures ? 1 : 0);
})().catch((e) => {
  // Recording unhandled rejections lets the page's own failures be reported,
  // but it also means a mistake in this file would end the run silently with
  // a success. It must not be possible to pass by not finishing.
  console.log("frontend smoke: the test itself threw\n" + (e && e.stack ? e.stack : e));
  process.exit(1);
});
