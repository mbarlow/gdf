"use strict";

let SESSION = null;
const choices = {}; // conflictId -> "ours"|"theirs"|"both"|"both-rev"|"base"|"none"

const $ = (id) => document.getElementById(id);

function escapeHTML(s) {
  return s.replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  })[c]);
}

// Flatten highlight.js HTML into a per-character class array, then overlay the
// intra-line diff op as an extra class, then re-serialize into coalesced spans.
function getCharClasses(node, inherited, out) {
  for (const child of node.childNodes) {
    if (child.nodeType === Node.TEXT_NODE) {
      for (const ch of child.data) out.push({ ch, cls: inherited });
    } else if (child.nodeType === Node.ELEMENT_NODE) {
      const cls = child.className
        ? (inherited ? inherited + " " + child.className : child.className)
        : inherited;
      getCharClasses(child, cls, out);
    }
  }
}

function highlightChars(text, lang) {
  if (!text) return [];
  let html;
  if (lang && lang !== "plaintext" && window.hljs && hljs.getLanguage(lang)) {
    try { html = hljs.highlight(text, { language: lang, ignoreIllegals: true }).value; }
    catch { html = escapeHTML(text); }
  } else {
    html = escapeHTML(text);
  }
  const tpl = document.createElement("template");
  tpl.innerHTML = html;
  const chars = [];
  getCharClasses(tpl.content, "", chars);
  return chars;
}

// Render one cell: combine syntax classes with diff-op backgrounds.
function renderCell(cell, lang, side) {
  const td = document.createElement("div");
  td.className = "code " + side;
  if (!cell) return td;

  const text = cell.spans.map((s) => s.text).join("");
  const chars = highlightChars(text, lang);

  // walk spans to assign diff op per char index
  let idx = 0;
  for (const sp of cell.spans) {
    const op = sp.op;
    for (let k = 0; k < sp.text.length; k++) {
      if (idx < chars.length && op !== 0) {
        chars[idx].diff = op < 0 ? "tok-del" : "tok-ins";
      }
      idx++;
    }
  }

  // coalesce consecutive chars with identical (cls + diff)
  let html = "";
  let run = "", runCls = null;
  const flush = () => {
    if (run === "" && runCls === null) return;
    const full = runCls || "";
    if (full) html += `<span class="${full}">${escapeHTML(run)}</span>`;
    else html += escapeHTML(run);
    run = "";
  };
  for (const c of chars) {
    const full = [c.cls, c.diff].filter(Boolean).join(" ");
    if (full !== runCls) { flush(); runCls = full; }
    run += c.ch;
  }
  flush();
  td.innerHTML = html || "";
  return td;
}

function gutter(n, side) {
  const g = document.createElement("div");
  g.className = "gutter " + side;
  g.textContent = n ? String(n) : "";
  return g;
}

function conflictBar(c) {
  const bar = document.createElement("div");
  bar.className = "cbar";
  bar.dataset.conflict = c.id;

  const label = document.createElement("span");
  label.className = "label";
  label.innerHTML = `<span class="dot"></span> conflict #${c.id}`;
  bar.appendChild(label);

  const spacer = document.createElement("span");
  spacer.className = "spacer";
  bar.appendChild(spacer);

  const opts = [
    ["ours", "ours"],
    ["theirs", "theirs"],
    ["both", "both ↓"],
    ["both-rev", "both ↑"],
  ];
  if (c.hasBase) opts.push(["base", "base"]);
  opts.push(["none", "skip"]);

  for (const [val, txt] of opts) {
    const b = document.createElement("button");
    b.className = "choice";
    b.textContent = txt;
    b.dataset.val = val;
    b.onclick = () => setChoice(c.id, val);
    bar.appendChild(b);
  }
  return bar;
}

function setChoice(id, val) {
  choices[id] = val;
  // update buttons
  document.querySelectorAll(`.cbar[data-conflict="${id}"]`).forEach((bar) => {
    bar.dataset.resolved = "1";
    bar.querySelectorAll(".choice").forEach((b) => {
      b.classList.toggle("sel", b.dataset.val === val);
    });
  });
  // dim the side(s) not kept
  document.querySelectorAll(`.row.conflict[data-conflict="${id}"]`).forEach((row) => {
    row.classList.remove("dim-left", "dim-right");
    if (val === "ours") row.classList.add("dim-right");
    else if (val === "theirs") row.classList.add("dim-left");
    else if (val === "base") row.classList.add("dim-left", "dim-right");
    else if (val === "none") row.classList.add("dim-left", "dim-right");
  });
  updateStatus();
}

function updateStatus() {
  if (SESSION.mode !== "merge") return;
  const total = SESSION.conflicts.length;
  const done = SESSION.conflicts.filter((c) => choices[c.id]).length;
  $("status").textContent = `${done}/${total} resolved`;
  $("merge-btn").disabled = done < total;
}

function render(rows, lang) {
  rows = rows || SESSION.rows;
  lang = lang || SESSION.language;
  const diff = $("diff");
  diff.innerHTML = "";
  const seenConflict = new Set();

  for (const row of rows) {
    if (row.type === "conflict" && !seenConflict.has(row.conflictId)) {
      seenConflict.add(row.conflictId);
      const c = SESSION.conflicts.find((x) => x.id === row.conflictId);
      diff.appendChild(conflictBar(c));
    }

    const r = document.createElement("div");
    r.className = "row " + row.type;
    if (row.conflictId) r.dataset.conflict = row.conflictId;

    r.appendChild(gutter(row.left ? row.left.n : 0, "left"));
    r.appendChild(renderCell(row.left, lang, "left"));
    r.appendChild(gutter(row.right ? row.right.n : 0, "right"));
    r.appendChild(renderCell(row.right, lang, "right"));
    diff.appendChild(r);
  }
}

/* ---------- git multi-file mode ---------- */
function selectFile(idx) {
  const f = SESSION.files[idx];
  $("filename").textContent = f.path;
  $("lang").textContent = f.language;
  document.querySelectorAll(".fl-item").forEach((el, i) =>
    el.classList.toggle("sel", i === idx)
  );
  render(f.rows, f.language);
  $("diff").scrollTop = 0;
}

function buildFileList() {
  const nav = $("filelist");
  nav.innerHTML = "";
  SESSION.files.forEach((f, i) => {
    const item = document.createElement("div");
    item.className = "fl-item";
    item.innerHTML =
      `<span class="fl-status ${f.status}">${f.status}</span>` +
      `<span class="fl-path" title="${f.path}">${f.path}</span>` +
      `<span class="fl-counts"><span class="add">+${f.added}</span> <span class="del">-${f.removed}</span></span>`;
    item.onclick = () => selectFile(i);
    nav.appendChild(item);
  });
}

/* ---------- theme ---------- */
function applyTheme(t) {
  if (t === "auto") {
    t = window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
  }
  document.documentElement.dataset.theme = t;
}

/* ---------- actions ---------- */
async function doMerge() {
  const res = await fetch("/api/resolve", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ choices }),
  });
  if (res.ok) {
    $("status").textContent = "merged — applied";
    window.close();
  } else {
    $("status").textContent = "error: " + (await res.text());
  }
}

async function doAbort() {
  await fetch("/api/cancel", { method: "POST" });
  window.close();
}

async function doClose() {
  await fetch("/api/close", { method: "POST" });
  window.close();
}

function bind() {
  $("theme-toggle").onclick = () => {
    const cur = document.documentElement.dataset.theme;
    applyTheme(cur === "dark" ? "light" : "dark");
  };
  $("merge-btn").onclick = doMerge;
  $("abort-btn").onclick = doAbort;
  $("quick-ours").onclick = () => SESSION.conflicts.forEach((c) => setChoice(c.id, "ours"));
  $("quick-theirs").onclick = () => SESSION.conflicts.forEach((c) => setChoice(c.id, "theirs"));
  window.addEventListener("keydown", (e) => {
    if (e.key === "Escape") doAbort();
    if ((e.ctrlKey || e.metaKey) && e.key === "Enter" && !$("merge-btn").disabled) doMerge();
  });
}

async function init() {
  SESSION = await (await fetch("/api/session")).json();
  applyTheme(SESSION.theme || "auto");

  $("filename").textContent = SESSION.filename;
  $("lang").textContent = SESSION.language;
  $("left-label").textContent = SESSION.left.label;
  $("left-sub").textContent = SESSION.left.sub;
  $("right-label").textContent = SESSION.right.label;
  $("right-sub").textContent = SESSION.right.sub;

  bind();
  document.documentElement.dataset.mode = SESSION.mode;

  if (SESSION.mode === "diff") {
    $("merge-btn").textContent = "Close";
    $("merge-btn").onclick = doClose;
    $("merge-btn").disabled = false;
    $("abort-btn").style.display = "none";
    $("quick-ours").style.display = "none";
    $("quick-theirs").style.display = "none";
    const ins = SESSION.rows.filter((r) => r.type === "insert").length;
    const del = SESSION.rows.filter((r) => r.type === "delete").length;
    const chg = SESSION.rows.filter((r) => r.type === "change").length;
    $("status").textContent = `+${ins}  -${del}  ~${chg}`;
    render();
  } else if (SESSION.mode === "git") {
    $("merge-btn").textContent = "Close";
    $("merge-btn").onclick = doClose;
    $("merge-btn").disabled = false;
    $("abort-btn").style.display = "none";
    $("quick-ours").style.display = "none";
    $("quick-theirs").style.display = "none";
    const add = SESSION.files.reduce((s, f) => s + f.added, 0);
    const del = SESSION.files.reduce((s, f) => s + f.removed, 0);
    $("status").textContent = `${SESSION.files.length} files  +${add}  -${del}`;
    buildFileList();
    selectFile(0);
  } else {
    render();
  }

  updateStatus();
}

init();
