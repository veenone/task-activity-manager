// applyTheme resolves the preference ("system" follows the OS) and sets the
// data-theme attribute the CSS tokens key off.
export function applyTheme(theme: string) {
  const dark =
    theme === "dark" ||
    (theme === "system" &&
      window.matchMedia?.("(prefers-color-scheme: dark)").matches);
  document.documentElement.dataset.theme = dark ? "dark" : "light";
}
