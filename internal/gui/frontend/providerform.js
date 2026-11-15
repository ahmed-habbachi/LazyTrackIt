// The provider dialog: adds a provider to config.yaml (gui.Service.
// AddProvider) or edits one (UpdateProvider; the name can't change). Which
// fields show depends on the type (service.ProviderTypes).
import { api, errorMessage } from "./api.js";
import { $, h } from "./dom.js";
import { button, closeModal, openModal } from "./modal.js";

const FIELDS = {
  baseURL: { label: "Base URL", placeholder: "https://trackit.yourcompany.com" },
  issuer: { label: "Login issuer", placeholder: "https://auth.yourcompany.com/realms/yourrealm" },
  clientID: { label: "Client ID", placeholder: "lazytrackit" },
  clientSecret: { label: "Client secret", placeholder: "Only for confidential clients", secret: true },
  scopes: { label: "Scopes", placeholder: "openid profile offline_access" },
  apiToken: { label: "API token", placeholder: "Paste the token", secret: true },
};

let types = [];

/**
 * @param {(result: {name: string, reload: boolean}) => void} onDone called
 *   after saving; reload is true when the app must switch to / reload name
 * @param {string} [editName] the provider to edit; omitted to add one
 */
export async function openProviderForm(onDone, editName) {
  const editing = !!editName;
  const title = editing ? `Edit provider “${editName}”` : "Add a provider";
  const values = { name: "", attendance: false };
  try {
    if (!types.length) types = await api.providerTypes();
    if (editing) {
      const d = await api.providerDraft(editName);
      Object.assign(values, d, { scopes: (d.scopes || []).join(" ") });
    }
  } catch (err) {
    openModal({ title, body: [h("p", { class: "form-error" }, errorMessage(err))], actions: [button("Close", "secondary", closeModal)] });
    return;
  }
  let type = types.find((t) => t.id === values.type) || types[0];

  const typeSelect = h("select", { id: "pf-type" }, types.map((t) => h("option", { value: t.id }, t.label)));
  const fieldsBox = h("div", { class: "pf-fields" });
  const hint = h("p", { class: "pf-hint" });
  const error = h("p", { class: "form-error", role: "alert" });
  typeSelect.value = type.id;
  const nameInput = h("input", {
    id: "pf-name", type: "text", maxlength: "40", autocomplete: "off", spellcheck: "false",
    placeholder: "e.g. trackit-test", value: values.name, disabled: editing,
    title: editing ? "A provider's name can't be changed" : undefined,
    oninput: (e) => { values.name = e.target.value; error.textContent = ""; },
  });

  const save = button(editing ? "Save changes" : "Add provider", "primary", submit);

  // applyDefaults fills the type's defaults (e.g. a TrackIt login copied
  // from an existing TrackIt provider) into fields that are still empty.
  async function applyDefaults() {
    try {
      const d = await api.newProviderDraft(type.id);
      for (const key of ["issuer", "clientID"]) if (!values[key] && d[key]) values[key] = d[key];
      if (!values.scopes && d.scopes && d.scopes.length) values.scopes = d.scopes.join(" ");
    } catch { /* defaults are only a convenience */ }
  }

  function renderFields() {
    const rows = [];
    for (const key of type.fields) {
      if (key === "attendance") {
        rows.push(h("label", { class: "setting toggle pf-toggle" },
          h("span", { class: "setting-label" }, "Check-in / check-out",
            h("span", { class: "pf-sub" }, "TrackIt 4.0.0 or newer")),
          h("input", {
            type: "checkbox", role: "switch", checked: values.attendance,
            onchange: (e) => { values.attendance = e.target.checked; },
          })));
        continue;
      }
      const f = FIELDS[key];
      let placeholder = f.placeholder;
      if (key === "baseURL" && type.baseURLOptional) placeholder = "Optional: leave empty for the default";
      rows.push(h("label", { class: "field" }, h("span", {}, f.label),
        h("input", {
          type: f.secret ? "password" : "text", autocomplete: "off", spellcheck: "false",
          placeholder, value: values[key] || "",
          oninput: (e) => { values[key] = e.target.value; error.textContent = ""; },
        })));
    }
    fieldsBox.replaceChildren(...rows);
    hint.textContent = type.id !== "trackit" ? ""
      : editing ? "Changing the login issuer or client ID signs you out of it, so you sign in again."
        : "You'll sign in to it in your browser right after adding it.";
    hint.hidden = !hint.textContent;
  }

  typeSelect.addEventListener("change", async () => {
    type = types.find((t) => t.id === typeSelect.value) || types[0];
    error.textContent = "";
    await applyDefaults();
    renderFields();
  });

  async function submit() {
    save.disabled = true;
    error.textContent = "";
    const draft = {
      name: values.name, type: type.id,
      baseURL: values.baseURL || "", issuer: values.issuer || "",
      clientID: values.clientID || "", clientSecret: values.clientSecret || "",
      scopes: (values.scopes || "").split(/[\s,]+/).filter(Boolean),
      apiToken: values.apiToken || "", attendance: !!values.attendance,
    };
    try {
      let result;
      if (editing) {
        draft.name = editName;
        result = { name: editName, reload: await api.updateProvider(draft) };
      } else {
        result = { name: await api.addProvider(draft), reload: true };
      }
      closeModal();
      onDone(result);
    } catch (err) {
      error.textContent = errorMessage(err);
      save.disabled = false;
    }
  }

  const form = h("div", {
    class: "pf",
    onkeydown: (e) => {
      if (e.key === "Enter" && e.target.matches("input:not([type=checkbox])")) { e.preventDefault(); submit(); }
    },
  },
  h("div", { class: "pf-row" },
    h("label", { class: "field" }, h("span", {}, "Name"), nameInput),
    h("label", { class: "field" }, h("span", {}, "Type"), typeSelect)),
  fieldsBox, hint, error);

  if (!editing) await applyDefaults();
  renderFields();
  openModal({
    title,
    body: [form],
    actions: [button("Cancel", "secondary", closeModal), save],
    kind: "provider",
  });
  (editing ? typeSelect : $("pf-name")).focus();
}
