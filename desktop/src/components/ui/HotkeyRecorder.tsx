import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { accelFromEvent, applyHotkey, currentHotkey, formatAccel } from "../../utils/hotkey";

interface HotkeyRecorderProps {
  /** Bump to re-sync the displayed value from storage (e.g. after restore-defaults). */
  syncKey?: number;
}

/**
 * HotkeyRecorder — a read-only input that captures a key combination on
 * focus and registers it as the global unlock hotkey. Esc cancels; a valid
 * combo (modifier + key) is applied immediately. Registration failures
 * (invalid or taken by another app) show an inline error and keep the old
 * hotkey. No animations — this is a keyboard-driven control.
 */
export function HotkeyRecorder({ syncKey = 0 }: HotkeyRecorderProps) {
  const { t } = useTranslation();
  const [display, setDisplay] = useState(() => formatAccel(currentHotkey()));
  const [recording, setRecording] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setDisplay(formatAccel(currentHotkey()));
    setError(null);
  }, [syncKey]);

  const handleKey = async (e: React.KeyboardEvent<HTMLInputElement>) => {
    e.preventDefault();
    if (e.key === "Escape") {
      e.currentTarget.blur();
      return;
    }
    const accel = accelFromEvent(e.nativeEvent);
    if (!accel) return; // modifier-only or unsupported — keep recording
    try {
      await applyHotkey(accel);
      setDisplay(formatAccel(accel));
      setError(null);
      e.currentTarget.blur();
    } catch {
      setError(t("manage.hotkeyError"));
    }
  };

  return (
    <>
      <input
        className={`text-input hotkey-recorder${recording ? " hotkey-recorder--recording" : ""}`}
        readOnly
        value={recording ? t("manage.hotkeyRecording") : display}
        onFocus={() => {
          setRecording(true);
          setError(null);
        }}
        onBlur={() => setRecording(false)}
        onKeyDown={(e) => void handleKey(e)}
      />
      {error && <div className="field__error">{error}</div>}
    </>
  );
}
