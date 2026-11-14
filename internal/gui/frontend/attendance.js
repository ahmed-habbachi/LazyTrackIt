// Check-in/out: the rail card and its dialogs.
import { api, errorMessage, isStale, needsReconnect, requestReconnect } from "./api.js";
import { $, h, formatMinutes, formatTime } from "./dom.js";
import { button, closeModal, openModal } from "./modal.js";
import { icon } from "./icons.js";

let current = null; // open session, or null when checked out

// Keep the elapsed time on the card ticking.
setInterval(() => { if (current) renderAttendance(current); }, 60 * 1000);

/** @param {boolean} enabled whether the active provider supports attendance */
export function setAttendanceEnabled(enabled) {
  $("attendance-card").hidden = !enabled;
  if (!enabled) current = null;
}

/** @param {{location: string, checkIn: string, note: string} | null} session */
export function renderAttendance(session) {
  current = session;
  const card = $("attendance-card");
  card.classList.toggle("in", !!session);
  if (session) {
    const since = new Date(session.checkIn);
    const elapsed = (Date.now() - since.getTime()) / 60000;
    $("attendance-status").replaceChildren(session.location,
      h("span", { class: "attendance-since" }, `since ${formatTime(since)} · ${formatMinutes(elapsed)}`));
    $("attendance-btn").replaceChildren(icon("log-out"), "Check out");
  } else {
    $("attendance-status").textContent = "Checked out";
    $("attendance-btn").replaceChildren(icon("log-in"), "Check in");
  }
}

export async function refreshAttendance() {
  try {
    renderAttendance(await api.attendanceStatus());
  } catch (err) {
    if (needsReconnect(err)) requestReconnect();
    else if (!isStale(err)) $("attendance-status").textContent = "Attendance unavailable";
  }
}

export function openAttendance() {
  if (current) openCheckOut();
  else openCheckIn();
}

function noteField() {
  return h("label", { class: "field" }, h("span", {}, "Note"),
    h("input", { id: "att-note", type: "text", maxlength: "500", placeholder: "optional" }));
}

function errorLine() {
  return h("p", { class: "form-error", id: "att-error", role: "alert" });
}

async function submit(btn, action) {
  btn.disabled = true;
  try {
    await action($("att-note").value);
    closeModal();
  } catch (err) {
    if (isStale(err)) { closeModal(); return; }
    if (needsReconnect(err)) { closeModal(); requestReconnect(); return; }
    $("att-error").textContent = errorMessage(err);
    btn.disabled = false;
  }
}

async function openCheckIn() {
  const list = h("div", { class: "locations" }, h("p", {}, h("span", { class: "spinner" }), "Loading work locations…"));
  const confirm = button("Check in", "primary", () => {
    const picked = list.querySelector("input:checked");
    if (picked) submit(confirm, (note) => api.checkIn(picked.value, note).then(renderAttendance));
  });
  confirm.disabled = true;
  openModal({ title: "Check in", body: [list, noteField(), errorLine()], actions: [button("Cancel", "ghost", closeModal), confirm] });

  try {
    const opts = await api.checkInOptions();
    list.replaceChildren(...opts.locations.map((loc, i) =>
      h("label", { class: "location" },
        h("input", { type: "radio", name: "location", value: loc.id, checked: i === opts.selected }),
        loc.name)));
    confirm.disabled = false;
  } catch (err) {
    if (isStale(err)) { closeModal(); return; }
    list.replaceChildren();
    $("att-error").textContent = errorMessage(err);
  }
}

function openCheckOut() {
  const since = new Date(current.checkIn);
  const elapsed = (Date.now() - since.getTime()) / 60000;
  const confirm = button("Check out", "primary", () =>
    submit(confirm, (note) => api.checkOut(note).then(() => renderAttendance(null))));
  openModal({
    title: "Check out",
    body: [h("p", {}, `Checked in at ${current.location} since ${formatTime(since)} (${formatMinutes(elapsed)}).`), noteField(), errorLine()],
    actions: [button("Cancel", "ghost", closeModal), confirm],
  });
}
