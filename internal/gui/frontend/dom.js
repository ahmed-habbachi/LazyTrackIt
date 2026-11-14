// DOM and formatting helpers. Everything user- or server-provided is set via
// textContent / attributes, never parsed as HTML.

export const $ = (id) => document.getElementById(id);

// h builds an element: h("div", { class: "x", onclick: fn }, child, "text").
export function h(tag, attrs = {}, ...children) {
  const el = document.createElement(tag);
  for (const [key, value] of Object.entries(attrs)) {
    if (value === undefined || value === null || value === false) continue;
    if (key.startsWith("on") && typeof value === "function") {
      el.addEventListener(key.slice(2), value);
    } else if (key === "style" && typeof value === "object") {
      for (const [prop, v] of Object.entries(value)) el.style.setProperty(prop, v);
    } else {
      el.setAttribute(key, value === true ? "" : String(value));
    }
  }
  for (const child of children.flat()) {
    if (child === undefined || child === null || child === false) continue;
    el.append(child instanceof Node ? child : document.createTextNode(String(child)));
  }
  return el;
}

const pad = (n) => String(n).padStart(2, "0");

// formatMinutes renders a duration as "7h05".
export function formatMinutes(total) {
  const sign = total < 0 ? "-" : "";
  const m = Math.abs(Math.round(total));
  return `${sign}${Math.floor(m / 60)}h${pad(m % 60)}`;
}

// clock renders minutes after midnight as "HH:MM" (1440 as "24:00").
export function clock(minutes) {
  return `${pad(Math.floor(minutes / 60))}:${pad(minutes % 60)}`;
}

// timeInputValue is clock() for <input type="time">, which can't hold 24:00.
export function timeInputValue(minutes) {
  return clock(minutes % 1440);
}

// parseTimeInput turns "HH:MM" into minutes. An end time of 00:00 after a
// non-zero start means midnight at the end of the day.
export function parseTimeInput(value, isEnd = false, start = 0) {
  if (!/^\d{2}:\d{2}$/.test(value)) return NaN;
  const [hh, mm] = value.split(":").map(Number);
  const minutes = hh * 60 + mm;
  return isEnd && minutes === 0 && start > 0 ? 1440 : minutes;
}

// Dates cross the Go boundary as "YYYY-MM-DD" and are handled here as local
// calendar dates, never through UTC parsing, so they can't shift a day.
export function parseDate(iso) {
  const [y, m, d] = iso.split("-").map(Number);
  return new Date(y, m - 1, d);
}

export function isoDate(date) {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

export function addDays(iso, days) {
  const d = parseDate(iso);
  d.setDate(d.getDate() + days);
  return isoDate(d);
}

const fmt = (opts) => new Intl.DateTimeFormat(undefined, opts);
const weekday = fmt({ weekday: "long" });
const dayMonth = fmt({ day: "numeric", month: "short" });
const dayMonthYear = fmt({ day: "numeric", month: "short", year: "numeric" });
const timeOfDay = fmt({ hour: "2-digit", minute: "2-digit" });

export const formatWeekday = (iso) => weekday.format(parseDate(iso));
export const formatDayMonth = (iso) => dayMonth.format(parseDate(iso));
export const formatRange = (start, end) => `${dayMonth.format(parseDate(start))} – ${dayMonthYear.format(parseDate(end))}`;
export const formatTime = (date) => timeOfDay.format(date);
