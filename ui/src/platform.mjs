export const isWindows = /Win/i.test(globalThis.navigator?.platform || "");
export const isMac = /Mac/i.test(globalThis.navigator?.platform || "");
export const isLinux = /^Linux\b/i.test(globalThis.navigator?.platform || "") &&
  !/Android|CrOS/i.test(globalThis.navigator?.userAgent || "");
export const usesControl = !isMac;
export const primaryModifier = (event, control = usesControl) =>
  control ? event.ctrlKey && !event.altKey && !event.metaKey : event.metaKey;
export const terminalModifier = (event, control = usesControl) =>
  control ? primaryModifier(event, true) && event.shiftKey : event.metaKey;
// In a normal shell, Ctrl+C is a signal until the user selects text.
export function shellClipboardShortcut(event, hasSelection, control = usesControl) {
  if (!control || !primaryModifier(event, true) || event.shiftKey || event.isComposing || event.keyCode === 229) return null;
  const key = event.key.toLowerCase();
  if (key === "v") return "paste";
  if (key === "c" && hasSelection) return "copy";
  return null;
}

export function shortcut(mac, control = usesControl) {
  return control
    ? mac.replaceAll("⌘", "Ctrl+").replaceAll("⇧", "Shift+").replaceAll("↵", "Enter")
    : mac;
}
