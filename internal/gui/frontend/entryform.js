// The entry drawer: create and edit time entries.
import { api, errorMessage, isStale, needsReconnect, requestReconnect } from "./api.js";
import { $, h, clock, formatMinutes, parseTimeInput, timeInputValue } from "./dom.js";
import { confirmDialog } from "./modal.js";

let draft = null;        // the entry being edited (gui.Draft shape)
let selectedTags = [];   // tag IDs
let tagsRequest = 0;     // drops tag responses for a project no longer selected
let afterChange = () => {};
let projects = [];
let inFlight = false;    // a save/delete is running; ignore repeats

/**
 * @param {{ onChange: () => void }} opts called after a successful save or delete
 */
export function initEntryForm({ onChange }) {
  afterChange = onChange;
  $("entry-form").addEventListener("submit", (e) => { e.preventDefault(); save(); });
  // Ctrl/Cmd+Enter saves from anywhere in the form, including the textarea.
  $("entry-form").addEventListener("keydown", (e) => {
    if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) { e.preventDefault(); save(); }
  });
  $("drawer-close").addEventListener("click", closeDrawer);
  $("f-cancel").addEventListener("click", closeDrawer);
  $("drawer-scrim").addEventListener("click", closeDrawer);
  $("f-delete").addEventListener("click", remove);
  $("f-project").addEventListener("change", () => { selectedTags = []; loadTags(); });
  for (const id of ["f-from", "f-to"]) $(id).addEventListener("input", renderDuration);
}

/** @param {{id: string, name: string, client: string}[]} list */
export function setProjects(list) {
  projects = list;
}

export const drawerOpen = () => $("drawer").classList.contains("open");

/** Opens the drawer for a new entry in the week containing day. */
export async function openNew(day, date) {
  try {
    const d = await api.newDraft(day);
    if (date) d.date = date;
    open(d, false);
  } catch (err) {
    showError(errorMessage(err));
  }
}

/** Opens the drawer to edit entry (a gui.Entry). */
export function openEdit(entry) {
  open({
    id: entry.id, projectId: entry.projectId, date: entry.date,
    from: entry.from, to: entry.to, description: entry.description, tags: entry.tagIds,
  }, true);
}

function open(d, isEdit) {
  draft = d;
  selectedTags = [...(d.tags || [])];
  $("drawer-title").textContent = isEdit ? "Edit entry" : "New entry";
  $("f-delete").hidden = !isEdit;
  $("f-error").textContent = "";

  const select = $("f-project");
  select.replaceChildren(...projects.map((p) =>
    h("option", { value: p.id }, p.client ? `${p.name} — ${p.client}` : p.name)));
  select.value = projects.some((p) => p.id === d.projectId) ? d.projectId : (projects[0]?.id ?? "");

  $("f-description").value = d.description || "";
  $("f-date").value = d.date;
  $("f-from").value = timeInputValue(d.from);
  $("f-to").value = timeInputValue(d.to);
  renderDuration();
  loadTags();

  $("drawer-scrim").hidden = false;
  $("drawer").classList.add("open");
  $("drawer").setAttribute("aria-hidden", "false");
  setTimeout(() => $(isEdit ? "f-description" : "f-project").focus(), 50);
}

export function closeDrawer() {
  $("drawer").classList.remove("open");
  $("drawer").setAttribute("aria-hidden", "true");
  $("drawer-scrim").hidden = true;
  draft = null;
}

function times() {
  const from = parseTimeInput($("f-from").value);
  return { from, to: parseTimeInput($("f-to").value, true, from) };
}

function renderDuration() {
  const { from, to } = times();
  const out = $("f-duration");
  if (Number.isNaN(from) || Number.isNaN(to)) out.textContent = "";
  else if (to <= from) out.textContent = "⚠ To must be after From";
  else out.textContent = `${clock(from)} → ${clock(to)} · ${formatMinutes(to - from)}`;
}

async function loadTags() {
  const projectId = $("f-project").value;
  const box = $("f-tags");
  const req = ++tagsRequest;
  if (!projectId) {
    box.replaceChildren(h("span", { class: "muted" }, "No projects available."));
    return;
  }
  box.replaceChildren(h("span", { class: "muted" }, h("span", { class: "spinner" }), "Loading tags…"));
  try {
    const tags = await api.tags(projectId);
    if (req !== tagsRequest) return;
    if (tags.length === 0) {
      box.replaceChildren(h("span", { class: "muted" }, "This project has no tags."));
      return;
    }
    box.replaceChildren(...tags.map((t) => {
      const chip = h("button", {
        type: "button", class: t.isSystem ? "chip system" : "chip",
        "aria-pressed": selectedTags.includes(t.id) ? "true" : "false",
        title: t.isSystem ? "System tag" : "Project tag",
      }, t.name);
      chip.addEventListener("click", () => {
        const on = !selectedTags.includes(t.id);
        selectedTags = on ? [...selectedTags, t.id] : selectedTags.filter((id) => id !== t.id);
        chip.setAttribute("aria-pressed", on ? "true" : "false");
      });
      return chip;
    }));
  } catch (err) {
    if (req !== tagsRequest || isStale(err)) return;
    box.replaceChildren(h("span", { class: "form-error" }, `Couldn't load tags: ${errorMessage(err)}`));
  }
}

async function save() {
  if (!draft || inFlight) return;
  const { from, to } = times();
  if (Number.isNaN(from) || Number.isNaN(to) || !$("f-date").value) {
    $("f-error").textContent = "Fill in the date and both times.";
    return;
  }
  const payload = {
    ...draft,
    projectId: $("f-project").value,
    description: $("f-description").value,
    date: $("f-date").value,
    from,
    to,
    tags: selectedTags,
  };
  await withBusy($("f-save"), async () => {
    await api.save(payload);
    closeDrawer();
    afterChange(payload.id ? "Entry updated." : "Entry created.");
  });
}

async function remove() {
  if (!draft?.id || inFlight) return;
  const id = draft.id;
  const ok = await confirmDialog("Delete entry?", "This time entry will be removed from the server.", "Delete");
  if (!ok) return;
  await withBusy($("f-delete"), async () => {
    await api.remove(id);
    closeDrawer();
    afterChange("Deleted.");
  });
}

async function withBusy(btn, action) {
  inFlight = true;
  btn.disabled = true;
  $("f-error").textContent = "";
  try {
    await action();
  } catch (err) {
    if (isStale(err)) closeDrawer();
    else if (needsReconnect(err)) { closeDrawer(); requestReconnect(); }
    else $("f-error").textContent = errorMessage(err);
  } finally {
    inFlight = false;
    btn.disabled = false;
  }
}

function showError(message) {
  const banner = $("error");
  banner.textContent = message;
  banner.hidden = !message;
}
