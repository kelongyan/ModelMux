import { ConfigProvider } from "antd";
import { createContext, useContext, useEffect, useMemo, useRef, useState } from "react";

import { createAppTheme, type AppThemeMode } from "./app-theme";

const THEME_STORAGE_KEY = "modelmux-theme";
let themeTransitionTimer: number | undefined;
let themeFadeTimer: number | undefined;
let themeFadeOverlay: HTMLDivElement | undefined;

type ThemeModeContextValue = {
  mode: AppThemeMode;
  isDark: boolean;
  setMode: (mode: AppThemeMode) => void;
  toggleMode: () => void;
};

const ThemeModeContext = createContext<ThemeModeContextValue | null>(null);

type AppThemeProviderProps = {
  children: React.ReactNode;
};

export function AppThemeProvider({ children }: AppThemeProviderProps): JSX.Element {
  const [mode, setModeState] = useState<AppThemeMode>(() => getInitialThemeMode());
  const [hasStoredPreference, setHasStoredPreference] = useState(() => getStoredThemeMode() !== null);
  const mounted = useRef(false);

  useEffect(() => {
    applyThemeMode(mode, mounted.current);
    mounted.current = true;
    if (hasStoredPreference) {
      saveThemeMode(mode);
    }
  }, [hasStoredPreference, mode]);

  useEffect(() => {
    if (hasStoredPreference || typeof window === "undefined") {
      return undefined;
    }
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => setModeState(media.matches ? "dark" : "light");
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, [hasStoredPreference]);

  const themeConfig = useMemo(() => createAppTheme(mode), [mode]);
  const contextValue = useMemo<ThemeModeContextValue>(() => ({
    mode,
    isDark: mode === "dark",
    setMode: (nextMode) => {
      setHasStoredPreference(true);
      setModeState(nextMode);
    },
    toggleMode: () => {
      setHasStoredPreference(true);
      setModeState((current) => (current === "dark" ? "light" : "dark"));
    },
  }), [mode]);

  return (
    <ThemeModeContext.Provider value={contextValue}>
      <ConfigProvider theme={themeConfig}>{children}</ConfigProvider>
    </ThemeModeContext.Provider>
  );
}

export function useThemeMode(): ThemeModeContextValue {
  const context = useContext(ThemeModeContext);
  if (!context) {
    throw new Error("useThemeMode must be used inside AppThemeProvider");
  }
  return context;
}

function getInitialThemeMode(): AppThemeMode {
  const stored = getStoredThemeMode();
  if (stored) {
    applyThemeMode(stored);
    return stored;
  }
  const systemMode = getSystemThemeMode();
  applyThemeMode(systemMode);
  return systemMode;
}

function getStoredThemeMode(): AppThemeMode | null {
  if (typeof window === "undefined") {
    return null;
  }
  try {
    const stored = window.localStorage.getItem(THEME_STORAGE_KEY);
    return stored === "dark" || stored === "light" ? stored : null;
  } catch {
    return null;
  }
}

function saveThemeMode(mode: AppThemeMode): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, mode);
  } catch {
    // Ignore storage failures; the in-memory theme still applies for this session.
  }
}

function getSystemThemeMode(): AppThemeMode {
  if (typeof window === "undefined") {
    return "light";
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function applyThemeMode(mode: AppThemeMode, animate = false): void {
  if (typeof document === "undefined") {
    return;
  }
  const root = document.documentElement;
  if (animate && shouldAnimateTheme()) {
    startThemeSoftFade(root);
    root.classList.add("theme-transition");
    if (themeTransitionTimer) {
      window.clearTimeout(themeTransitionTimer);
    }
    themeTransitionTimer = window.setTimeout(() => {
      root.classList.remove("theme-transition");
      themeTransitionTimer = undefined;
    }, 260);
  }
  root.dataset.theme = mode;
  root.style.colorScheme = mode;
}

function shouldAnimateTheme(): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  return window.matchMedia("(prefers-reduced-motion: no-preference)").matches;
}

function startThemeSoftFade(root: HTMLElement): void {
  if (typeof document === "undefined") {
    return;
  }

  themeFadeOverlay?.remove();
  if (themeFadeTimer) {
    window.clearTimeout(themeFadeTimer);
  }

  const styles = window.getComputedStyle(root);
  const overlay = document.createElement("div");
  overlay.className = "theme-soft-fade";
  overlay.style.setProperty("--theme-fade-bg", styles.getPropertyValue("--mm-bg").trim());
  overlay.style.setProperty("--theme-fade-surface", styles.getPropertyValue("--mm-surface-solid").trim());
  document.body.appendChild(overlay);
  themeFadeOverlay = overlay;

  themeFadeTimer = window.setTimeout(() => {
    overlay.remove();
    if (themeFadeOverlay === overlay) {
      themeFadeOverlay = undefined;
    }
    themeFadeTimer = undefined;
  }, 380);
}
