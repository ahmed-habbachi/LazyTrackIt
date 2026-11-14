// The single modal dialog, used for the login prompt, check in/out, delete
// confirmation and the first-run setup message.
import { $, h } from "./dom.js";

let onClose = null;
let currentKind = "";

/**
 * Opens the modal, replacing any open one (whose onDismiss runs first, so
 * e.g. a pending confirmation resolves instead of hanging).
 * @param {{ title: string, body: Node[], actions?: Node[], dismissible?: boolean, onDismiss?: () => void, kind?: string }} opts
 */
export function openModal({ title, body, actions = [], dismissible = true, onDismiss, kind = "" }) {
  const replaced = onClose;
  onClose = null;
  if (replaced) replaced();
  currentKind = kind;
  $("modal-title").textContent = title;
  $("modal-body").replaceChildren(...body);
  $("modal-foot").replaceChildren(...actions);
  $("modal").dataset.dismissible = dismissible ? "1" : "";
  onClose = onDismiss || null;
  $("modal").hidden = false;
  $("modal-scrim").hidden = false;
  const focusable = $("modal").querySelector("input, select, textarea, button.btn-primary, button");
  if (focusable) focusable.focus();
}

export function closeModal() {
  $("modal").hidden = true;
  $("modal-scrim").hidden = true;
  currentKind = "";
  const cb = onClose;
  onClose = null;
  if (cb) cb();
}

/** Closes the modal only if it's still the one of the given kind. */
export function closeModalOfKind(kind) {
  if (modalOpen() && currentKind === kind) closeModal();
}

export const modalOpen = () => !$("modal").hidden;

// dismissModal closes the modal on Escape / scrim click, unless the current
// one must stay up (e.g. the setup message).
export function dismissModal() {
  if (modalOpen() && $("modal").dataset.dismissible) closeModal();
}

export function button(label, variant, onclick) {
  return h("button", { type: "button", class: `btn btn-${variant}`, onclick }, label);
}

/**
 * Asks for confirmation; resolves true if confirmed.
 * @param {string} title
 * @param {string} message
 * @param {string} confirmLabel
 * @returns {Promise<boolean>}
 */
export function confirmDialog(title, message, confirmLabel) {
  return new Promise((resolve) => {
    let answered = false;
    const answer = (value) => {
      answered = true;
      closeModal();
      resolve(value);
    };
    openModal({
      title,
      body: [h("p", {}, message)],
      actions: [button("Cancel", "ghost", () => answer(false)), button(confirmLabel, "primary", () => answer(true))],
      onDismiss: () => { if (!answered) resolve(false); },
    });
  });
}
