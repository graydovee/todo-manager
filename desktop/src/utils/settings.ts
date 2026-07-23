/** Centralized defaults for user-configurable settings (localStorage-backed). */

export const DEFAULT_LOCK_OPACITY = 0.75;

export const DEFAULT_FILTER_STATUS = ["open", "in_progress"];

export const DEFAULT_UNLOCK_HOTKEY = "Alt+Shift+F11";

/** Default UI language follows the OS locale (zh for Chinese, else English). */
export function detectLang(): string {
  return navigator.language.startsWith("zh") ? "zh" : "en";
}
