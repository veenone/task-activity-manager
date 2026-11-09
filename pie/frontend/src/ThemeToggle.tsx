import { applyTheme } from "@agile-suite/core";
import { SetTheme } from "../wailsjs/go/main/App";

// ThemeToggle flips between light and dark from whatever is showing, which may
// be the OS's choice. View > Theme also offers System.
export function ThemeToggle({ shown, onChange }: { shown: string; onChange: (theme: string) => void }) {
  const next = shown === "dark" ? "light" : "dark";
  return (
    <button
      type="button"
      className="btn btn-ghost"
      aria-label={`Switch to ${next} theme`}
      title={`Switch to ${next} theme`}
      onClick={() => {
        applyTheme(next);
        onChange(next);
        void SetTheme(next);
      }}
    >
      <span aria-hidden="true">{next === "dark" ? "☾" : "☀"}</span>
    </button>
  );
}
