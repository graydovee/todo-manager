import { register, unregisterAll, type ShortcutEvent } from "@tauri-apps/plugin-global-shortcut";
import { DEFAULT_UNLOCK_HOTKEY } from "./settings";

/**
 * Global unlock hotkey — single registration owner.
 *
 * The OS-level shortcut is registered through tauri-plugin-global-shortcut.
 * App.tsx sets the unlock callback once via onUnlock(); the manage panel
 * re-registers via applyHotkey() when the user records a new combination.
 */
let unlockHandler: (() => void) | null = null;

/**
 * Shortcut registration handler. Only fires on the "Pressed" transition so
 * that holding the key down (which some platforms surface as repeated
 * Pressed/Released events) doesn't toggle the lock on and off repeatedly.
 */
function onShortcutPressed(event: ShortcutEvent) {
  if (event.state === "Pressed") unlockHandler?.();
}

/** Set the callback fired when the global hotkey is pressed. */
export function onUnlock(cb: () => void) {
  unlockHandler = cb;
}

/** The currently configured accelerator (falls back to the default). */
export function currentHotkey(): string {
  return localStorage.getItem("unlock_hotkey") || DEFAULT_UNLOCK_HOTKEY;
}

/**
 * Register `accel` as the global unlock hotkey and persist it.
 * On failure (invalid or grabbed by another app) the previous hotkey is
 * re-registered so the app always keeps a working one; the error is rethrown.
 */
export async function applyHotkey(accel: string): Promise<void> {
  const prev = currentHotkey();
  try {
    await unregisterAll();
    await register(accel, onShortcutPressed);
  } catch (e) {
    try {
      await unregisterAll();
      await register(prev, onShortcutPressed);
    } catch {
      /* keep the original error */
    }
    throw e;
  }
  localStorage.setItem("unlock_hotkey", accel);
}

/**
 * Convert a keydown event into a Tauri accelerator string
 * (e.g. "Alt+Shift+F11"). Returns null for modifier-only presses,
 * unsupported keys, and combinations without any modifier — a global
 * hotkey must include at least one modifier to avoid hijacking typing.
 */
export function accelFromEvent(e: KeyboardEvent): string | null {
  const key = mapKey(e.key);
  if (!key) return null;
  const mods: string[] = [];
  if (e.ctrlKey) mods.push("Control");
  if (e.altKey) mods.push("Alt");
  if (e.shiftKey) mods.push("Shift");
  if (e.metaKey) mods.push("Super");
  if (mods.length === 0) return null;
  return [...mods, key].join("+");
}

/** Display formatting: "alt+shift+f11" -> "Alt+Shift+F11". */
export function formatAccel(accel: string): string {
  return accel
    .split("+")
    .map((p) => (p ? p[0].toUpperCase() + p.slice(1) : p))
    .join("+");
}

function mapKey(k: string): string | null {
  if (/^[a-zA-Z]$/.test(k)) return k.toUpperCase();
  if (/^[0-9]$/.test(k)) return k;
  if (/^F([1-9]|1[0-9]|2[0-4])$/.test(k)) return k;
  const named: Record<string, string> = {
    " ": "Space",
    Enter: "Enter",
    Tab: "Tab",
    Backspace: "Backspace",
    Delete: "Delete",
    Insert: "Insert",
    Home: "Home",
    End: "End",
    PageUp: "PageUp",
    PageDown: "PageDown",
    ArrowUp: "Up",
    ArrowDown: "Down",
    ArrowLeft: "Left",
    ArrowRight: "Right",
  };
  return named[k] ?? null;
}
