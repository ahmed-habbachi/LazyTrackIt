// The Settings dialog: everything that isn't the week itself, in one place.
// Providers (switch, add, edit, remove), "Start at login", and versions.
import { api, errorMessage } from "./api.js";
import { $, h } from "./dom.js";
import { icon } from "./icons.js";
import { button, closeModal, confirmDialog, openModal } from "./modal.js";
import { openProviderForm } from "./providerform.js";

/**
 * What the dialog needs from app.js.
 * @typedef {{
 *   switchTo: (name: string) => Promise<void>,
 *   reloadActive: () => Promise<void>,
 *   refreshChrome: () => Promise<void>,
 *   flash: (message: string) => void,
 *   showError: (message: string) => void,
 *   versions: () => {app: string, backend: string, provider: string},
 * }} SettingsHooks
 */

/** @type {SettingsHooks} */
let hooks;

/** @param {SettingsHooks} h */
export function initSettings(h) {
  hooks = h;
}

/** Keeps the dialog's switch in step when autostart changes elsewhere (tray). */
export function syncAutostart(enabled) {
  const el = $("settings-autostart");
  if (el) el.checked = !!enabled;
}

export async function openSettings() {
  let list, st;
  try {
    [list, st] = await Promise.all([api.providerList(), api.state()]);
  } catch (err) {
    hooks.showError(errorMessage(err));
    return;
  }

  const only = list.length < 2;
  const rows = list.map((p) => h("div", { class: "pm-row" },
    h("div", { class: "pm-main" },
      h("div", { class: "pm-name" }, p.name),
      h("div", { class: "pm-type" }, p.type || "")),
    p.active
      ? h("span", { class: "pm-active" }, "Active")
      : h("button", {
        class: "btn btn-secondary btn-sm", type: "button", title: `Switch to ${p.name}`,
        onclick: () => { closeModal(); hooks.switchTo(p.name); },
      }, "Use"),
    h("button", {
      class: "icon-btn", type: "button", title: `Edit ${p.name}`, "aria-label": `Edit ${p.name}`,
      onclick: () => openProviderForm(async ({ name, reload }) => {
        hooks.flash(`Saved provider "${name}"`);
        if (reload) await hooks.reloadActive();
      }, p.name),
    }, icon("pencil")),
    h("button", {
      class: "icon-btn pm-remove", type: "button", disabled: only,
      title: only ? "You can't remove your only provider" : `Remove ${p.name}`, "aria-label": `Remove ${p.name}`,
      onclick: () => removeProvider(p.name, p.active),
    }, icon("trash"))));

  const autostart = h("input", {
    type: "checkbox", role: "switch", id: "settings-autostart", checked: st.autostart,
    onchange: async (e) => {
      const want = e.target.checked;
      try {
        await api.setAutostart(want);
      } catch (err) {
        e.target.checked = !want;
        hooks.showError(`Couldn't change "Start at login": ${errorMessage(err)}`);
      }
    },
  });

  const v = hooks.versions();
  const about = [h("div", { class: "about-row" }, h("span", {}, "LazyTrackIt"), h("span", {}, v.app))];
  if (v.backend) {
    about.push(h("div", { class: "about-row" }, h("span", {}, `${v.provider} server`), h("span", {}, v.backend)));
  }

  openModal({
    title: "Settings",
    body: [
      h("section", { class: "settings-section" },
        h("p", { class: "eyebrow" }, icon("user"), "Providers"),
        h("div", { class: "pm-list" }, rows),
        h("button", {
          class: "link-btn", type: "button",
          onclick: () => openProviderForm(async ({ name }) => {
            await hooks.switchTo(name);
            hooks.flash(`Added provider "${name}"`);
          }),
        }, icon("plus"), "Add a provider")),
      h("section", { class: "settings-section" },
        h("p", { class: "eyebrow" }, icon("settings"), "General"),
        h("label", { class: "setting toggle pf-toggle" },
          h("span", { class: "setting-label" }, "Start at login",
            h("span", { class: "pf-sub" }, "Opens LazyTrackIt in the tray when you log in")),
          autostart)),
      h("section", { class: "settings-section" },
        h("p", { class: "eyebrow" }, icon("clock"), "About"),
        h("div", { class: "about" }, about)),
    ],
    actions: [button("Done", "primary", closeModal)],
    kind: "settings",
  });
}

async function removeProvider(name, active) {
  const ok = await confirmDialog(`Remove “${name}”?`,
    "It's deleted from your config file, and its saved login is removed." +
      (active ? " It's the provider you're using, so LazyTrackIt switches to another one." : ""),
    "Remove");
  if (!ok) {
    openSettings();
    return;
  }
  try {
    const next = await api.removeProvider(name);
    if (next) {
      hooks.flash(`Removed "${name}", switched to "${next}"`);
      await hooks.reloadActive();
    } else {
      hooks.flash(`Removed "${name}"`);
      await hooks.refreshChrome();
      openSettings(); // back to the list
    }
  } catch (err) {
    hooks.showError(errorMessage(err));
  }
}
