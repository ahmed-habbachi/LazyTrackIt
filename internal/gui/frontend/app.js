// LazyTrackIt desktop frontend: boot, the week ledger, the rail, and wiring
// to Go events. Business rules live in Go (internal/service); this file only
// renders what the gui.Service returns.
import { api, errorMessage, isStale, needsReconnect, on } from "./api.js";
import {
  $, h, addDays, clock, formatDayMonth, formatMinutes, formatRange, formatWeekday,
} from "./dom.js";
import { hydrateIcons, icon } from "./icons.js";
import { closeModalOfKind, dismissModal, modalOpen, openModal } from "./modal.js";
import { closeDrawer, drawerOpen, initEntryForm, openEdit, openNew, setProjects } from "./entryform.js";
import { openAttendance, refreshAttendance, renderAttendance, setAttendanceEnabled } from "./attendance.js";

const state = {
  day: "",          // any day of the shown week ("" = current week)
  week: null,       // last gui.Week
  projects: [],
  colors: new Map(), // projectId -> color
  weekRequest: 0,   // drops responses for a week no longer shown
  startRequest: 0,  // drops results of a start() superseded by a newer one
  statusTimer: 0,
};

// ---------- boot ----------

async function boot() {
  hydrateIcons();
  initEntryForm({ onChange: (msg) => { flash(msg); loadWeek(); } });
  bindChrome();
  bindEvents();

  const st = await api.state();
  renderChrome(st);
  if (st.setupMessage) {
    openModal({ title: "Welcome to LazyTrackIt", body: [h("p", {}, st.setupMessage)], dismissible: false });
    return;
  }
  await start();
}

// start logs in (if needed) and loads the profile and current week.
async function start() {
  const req = ++state.startRequest;
  renderSkeleton();
  try {
    const projects = await api.start();
    if (req !== state.startRequest) return;
    closeLoginPrompt();
    state.projects = projects;
    state.colors = new Map(projects.map((p) => [p.id, p.color]));
    setProjects(projects);
    renderChrome(await api.state());
    await loadWeek({ afterStart: true });
  } catch (err) {
    if (isStale(err) || req !== state.startRequest) return;
    closeLoginPrompt();
    showError(errorMessage(err), start);
  }
}

// ---------- week ----------

// loadWeek shows the current week. If the login expired (or the profile
// never loaded) it runs start() instead, which logs in afresh, unless a
// successful start() itself just called it, so the two can never loop.
async function loadWeek({ afterStart = false } = {}) {
  const req = ++state.weekRequest;
  try {
    const week = await api.week(state.day);
    if (req !== state.weekRequest) return;
    state.week = week;
    state.day = week.start;
    showError("");
    renderWeek(week);
  } catch (err) {
    if (req !== state.weekRequest || isStale(err)) return;
    if (needsReconnect(err) && !afterStart) {
      start();
      return;
    }
    const message = needsReconnect(err) ? "Couldn't load your profile. Try again." : errorMessage(err);
    showError(message, () => loadWeek());
  }
}

function shiftWeek(days) {
  state.day = state.day ? addDays(state.day, days) : "";
  renderSkeleton();
  loadWeek();
}

function goToday() {
  state.day = "";
  renderSkeleton();
  loadWeek();
}

function renderWeek(week) {
  const isCurrent = week.today >= week.start && week.today <= week.end;
  $("this-week").textContent = isCurrent ? "This week" : formatDayMonth(week.start);
  $("range").textContent = formatRange(week.start, week.end);
  $("headline").textContent = isCurrent ? "This week" : `Week of ${formatDayMonth(week.start)}`;
  renderMeter(week);

  const byDay = new Map();
  for (const e of week.entries) {
    if (!byDay.has(e.date)) byDay.set(e.date, []);
    byDay.get(e.date).push(e);
  }

  const days = [];
  for (let i = 0; i < 7; i++) {
    const date = addDays(week.start, i);
    const entries = (byDay.get(date) || []).slice().sort((a, b) => a.from - b.from);
    const weekend = i >= 5;
    if (weekend && entries.length === 0) continue; // keep weekends out of the way unless used
    days.push(renderDay(date, entries, date === week.today, weekend));
  }
  $("days").replaceChildren(...days);
}

function renderDay(date, entries, isToday, weekend) {
  const total = entries.reduce((sum, e) => sum + e.minutes, 0);
  const cls = ["day", isToday && "today", weekend && "weekend"].filter(Boolean).join(" ");
  const add = () => openNew(state.day, date);
  return h("section", { class: cls, "aria-label": formatWeekday(date) },
    h("header", { class: "day-head" },
      h("span", { class: "day-name" }, formatWeekday(date)),
      h("span", { class: "day-date" }, formatDayMonth(date)),
      isToday && h("span", { class: "today-badge" }, "Today"),
      h("span", { class: "day-total" }, formatMinutes(total)),
      h("button", { class: "icon-btn day-add", type: "button", title: "Add an entry", "aria-label": `Add an entry on ${formatWeekday(date)}`, onclick: add },
        icon("plus"))),
    h("div", { class: "day-entries" },
      entries.map(renderEntry),
      entries.length === 0 && h("button", { class: "day-empty", type: "button", onclick: add },
        icon("plus"), "Log time")));
}

function renderEntry(e) {
  const color = state.colors.get(e.projectId);
  return h("button", {
    type: "button",
    class: e.editable ? "entry" : "entry locked",
    style: color ? { "--entry-color": color } : undefined,
    title: e.locked ? "This entry is locked and cannot be edited."
      : e.editable ? "Edit entry" : "This entry runs past midnight; edit it in the terminal UI or on the server.",
    onclick: () => { if (e.editable) openEdit(e); },
  },
  h("span", { class: "entry-time" }, `${clock(e.from)} – ${clock(e.to)}`),
  h("span", { class: "entry-main" },
    h("div", { class: "entry-project" },
      h("span", { class: "entry-project-name" }, e.projectName),
      e.locked && h("span", { class: "lock" }, icon("lock"), "Locked")),
    h("div", { class: "entry-desc" }, e.description || "—")),
  h("span", { class: "entry-dur" }, formatMinutes(e.minutes)));
}

function renderMeter(week) {
  const { loggedMinutes: logged, expectedMinutes: expected } = week;
  $("logged").textContent = formatMinutes(logged);
  $("expected").textContent = `of ${formatMinutes(expected)}`;

  const scale = Math.max(logged, expected, 40 * 60);
  $("meter-fill").style.width = `${Math.min(100, (logged / scale) * 100)}%`;
  $("meter-mark").style.left = `calc(${Math.min(100, (expected / scale) * 100)}% - 1px)`;

  const delta = logged - expected;
  const el = $("delta");
  el.className = "meter-delta" + (delta < 0 ? " behind" : delta > 0 ? " ahead" : "");
  el.textContent = delta < 0 ? `${formatMinutes(-delta)} behind` : delta > 0 ? `${formatMinutes(delta)} ahead` : "On track";
}

function renderSkeleton() {
  $("days").replaceChildren(...[0, 1, 2].map(() => h("div", { class: "skeleton" })));
}

// ---------- chrome ----------

function renderChrome(st) {
  $("member").textContent = st.member || (st.needsLogin ? "Not signed in" : " ");
  $("version").textContent = /^v?\d/.test(st.version) ? `v${st.version.replace(/^v/, "")}` : st.version;
  $("autostart").checked = st.autostart;
  setAttendanceEnabled(st.attendance);
  if (st.attendance) refreshAttendance();

  const providers = st.providers || [];
  $("provider-field").hidden = providers.length < 2;
  $("provider").replaceChildren(...providers.map((p) => h("option", { value: p }, p)));
  $("provider").value = st.active;
  $("provider").dataset.active = st.active;
  showUpdateNotice(st.updateNotice);
}

function showUpdateNotice(notice) {
  $("update-notice").dataset.persistent = notice || "";
  $("update-notice").textContent = notice || "";
  $("update-notice").hidden = !notice;
}

function showError(message, retry) {
  const banner = $("error");
  banner.replaceChildren(message ? message : "");
  if (message && retry) {
    banner.append(h("button", { class: "btn btn-secondary btn-sm", type: "button", onclick: retry }, icon("refresh-cw"), "Retry"));
  }
  banner.hidden = !message;
}

function flash(message) {
  clearTimeout(state.statusTimer);
  const notice = $("update-notice");
  const previous = notice.dataset.persistent || "";
  notice.replaceChildren(icon("circle-check"), message);
  notice.hidden = false;
  state.statusTimer = setTimeout(() => showUpdateNotice(previous), 4000);
}

// ---------- login prompt ----------

function showLoginPrompt(p) {
  const body = [];
  if (p.Message) body.push(h("p", {}, p.Message));
  if (p.Code) body.push(h("div", { class: "code", "aria-label": "Verification code" }, p.Code));
  if (p.URL) body.push(h("p", {}, "If your browser didn't open, visit ", h("span", { class: "link" }, p.URL)));
  body.push(h("p", {}, h("span", { class: "spinner" }), "Waiting for approval…"));
  openModal({ title: "Sign in to continue", body, dismissible: false, kind: "login" });
}

function closeLoginPrompt() {
  closeModalOfKind("login");
}

// ---------- wiring ----------

function bindChrome() {
  $("prev-week").addEventListener("click", () => shiftWeek(-7));
  $("next-week").addEventListener("click", () => shiftWeek(7));
  $("this-week").addEventListener("click", goToday);
  $("refresh").addEventListener("click", () => { loadWeek(); refreshAttendance(); });
  $("new-entry").addEventListener("click", () => openNew(state.day));
  $("attendance-btn").addEventListener("click", openAttendance);
  $("modal-scrim").addEventListener("click", dismissModal);

  $("autostart").addEventListener("change", async (e) => {
    const want = e.target.checked;
    try {
      await api.setAutostart(want);
    } catch (err) {
      e.target.checked = !want;
      showError(`Couldn't change "Start at login": ${errorMessage(err)}`);
    }
  });

  $("provider").addEventListener("change", async (e) => {
    const previous = e.target.dataset.active || "";
    try {
      await api.switchProvider(e.target.value);
      e.target.dataset.active = e.target.value;
      state.day = "";
      state.week = null;
      await start();
    } catch (err) {
      e.target.value = previous;
      showError(errorMessage(err));
    }
  });

  document.addEventListener("keydown", onKey);
  document.addEventListener("lt:reconnect", () => start());
}

function onKey(e) {
  if (e.key === "Escape") {
    if (modalOpen()) dismissModal();
    else if (drawerOpen()) closeDrawer();
    return;
  }
  const typing = e.target.closest("input, textarea, select");
  if (typing || modalOpen() || drawerOpen() || e.ctrlKey || e.metaKey || e.altKey) return;
  const actions = {
    n: () => openNew(state.day),
    r: () => { loadWeek(); refreshAttendance(); },
    t: goToday,
    c: () => { if (!$("attendance-card").hidden) openAttendance(); },
    ArrowLeft: () => shiftWeek(-7),
    ArrowRight: () => shiftWeek(7),
  };
  const action = actions[e.key];
  if (action) {
    e.preventDefault();
    action();
  }
}

function bindEvents() {
  on("login:prompt", showLoginPrompt);
  on("attendance", renderAttendance);
  on("update", showUpdateNotice);
  on("autostart", (enabled) => { $("autostart").checked = !!enabled; });
  on("navigate", (view) => {
    if (view === "attendance" && !$("attendance-card").hidden) openAttendance();
  });
  // Re-check the week when the window regains focus after a while (e.g. it
  // was hidden in the tray overnight), so it never shows a stale "today".
  let hiddenAt = 0;
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) hiddenAt = Date.now();
    else if (hiddenAt && Date.now() - hiddenAt > 5 * 60 * 1000 && state.week) {
      loadWeek();
      refreshAttendance();
    }
  });
}

boot().catch((err) => showError(errorMessage(err)));
