// Thin wrapper over the Go Service bound by Wails (internal/gui/service.go).
// Methods are called by their fully-qualified Go name, so no generated
// bindings or JS build step are needed.
import { Call, Events } from "/wails/runtime.js";

const SERVICE = "github.com/ahmed-habbachi/lazytrackit/internal/gui.Service";

// STALE mirrors gui.StaleMessage: the call raced a provider switch and its
// result belongs to a provider no longer shown, so callers drop it.
export const STALE = "stale";

export function isStale(err) {
  return errorMessage(err) === STALE;
}

// RECONNECT mirrors gui.ReconnectMessage: the login expired or the profile
// isn't loaded, so the app must run start() again (which logs in afresh).
export const RECONNECT = "reconnect";

export function needsReconnect(err) {
  return errorMessage(err) === RECONNECT;
}

// requestReconnect asks app.js to run start() again, from any module.
export function requestReconnect() {
  document.dispatchEvent(new CustomEvent("lt:reconnect"));
}

export function errorMessage(err) {
  if (!err) return "";
  if (typeof err === "string") return err;
  return err.message || String(err);
}

const call = (method, ...args) => Call.ByName(`${SERVICE}.${method}`, ...args);

export const api = {
  state: () => call("State"),
  start: () => call("Start"),
  switchProvider: (name) => call("SwitchProvider", name),
  week: (day) => call("Week", day),
  newDraft: (day) => call("NewDraft", day),
  tags: (projectId) => call("Tags", projectId),
  save: (draft) => call("Save", draft),
  remove: (id) => call("Delete", id),
  attendanceStatus: () => call("AttendanceStatus"),
  checkInOptions: () => call("CheckInOptions"),
  checkIn: (locationId, note) => call("CheckIn", locationId, note),
  checkOut: (note) => call("CheckOut", note),
  setAutostart: (enabled) => call("SetAutostart", enabled),
  providerTypes: () => call("ProviderTypes"),
  newProviderDraft: (type) => call("NewProviderDraft", type),
  addProvider: (draft) => call("AddProvider", draft),
  providerList: () => call("ProviderList"),
  backendVersion: () => call("BackendVersion"),
  providerDraft: (name) => call("ProviderDraft", name),
  updateProvider: (draft) => call("UpdateProvider", draft),
  removeProvider: (name) => call("RemoveProvider", name),
  restartToUpdate: () => call("RestartToUpdate"),
  downloadUpdate: () => call("DownloadUpdate"),
};

// on subscribes to a Go-emitted event, handing the callback its payload.
export function on(name, callback) {
  return Events.On(name, (event) => callback(event.data));
}
