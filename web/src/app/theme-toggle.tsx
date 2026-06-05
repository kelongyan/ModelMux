import { useThemeMode } from "./theme-mode";

export function ThemeToggle(): JSX.Element {
  const { isDark, toggleMode } = useThemeMode();

  return (
    <button
      type="button"
      className="theme-toggle"
      aria-label={isDark ? "切换到浅色模式" : "切换到深色模式"}
      title={isDark ? "切换到浅色模式" : "切换到深色模式"}
      onClick={toggleMode}
    >
      {isDark ? <SunIcon /> : <MoonIcon />}
    </button>
  );
}

function SunIcon(): JSX.Element {
  return (
    <svg className="theme-toggle-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2.5M12 19.5V22M4.93 4.93 6.7 6.7M17.3 17.3l1.77 1.77M2 12h2.5M19.5 12H22M4.93 19.07 6.7 17.3M17.3 6.7l1.77-1.77" />
    </svg>
  );
}

function MoonIcon(): JSX.Element {
  return (
    <svg className="theme-toggle-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M20.1 14.8A8.2 8.2 0 0 1 9.2 3.9a8.7 8.7 0 1 0 10.9 10.9Z" />
    </svg>
  );
}
