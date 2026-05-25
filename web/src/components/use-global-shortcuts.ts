import { useEffect, useRef } from "react";

type ShortcutHandlers = {
  onReload?: () => void;
  onGoto?: (path: string) => void;
};

// useGlobalShortcuts 监听 Ctrl/Cmd+R 重载与 vim 风格的 g <letter> 切页和弦。
export function useGlobalShortcuts({ onReload, onGoto }: ShortcutHandlers): void {
  const reloadRef = useRef(onReload);
  const gotoRef = useRef(onGoto);

  useEffect(() => {
    reloadRef.current = onReload;
    gotoRef.current = onGoto;
  }, [onReload, onGoto]);

  useEffect(() => {
    let chordTimer: number | null = null;
    let waitingForChord = false;

    const clearChord = () => {
      waitingForChord = false;
      if (chordTimer !== null) {
        window.clearTimeout(chordTimer);
        chordTimer = null;
      }
    };

    const handler = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (target && isEditableTarget(target)) {
        return;
      }

      if ((event.ctrlKey || event.metaKey) && !event.shiftKey && !event.altKey && event.key.toLowerCase() === "r") {
        const reload = reloadRef.current;
        if (reload) {
          event.preventDefault();
          reload();
        }
        return;
      }

      if (event.ctrlKey || event.metaKey || event.altKey) {
        return;
      }

      if (event.key === "g") {
        waitingForChord = true;
        if (chordTimer !== null) {
          window.clearTimeout(chordTimer);
        }
        chordTimer = window.setTimeout(clearChord, 1500);
        return;
      }

      if (!waitingForChord) {
        return;
      }

      const mapping: Record<string, string> = {
        d: "/dashboard",
        p: "/providers",
        s: "/settings",
        e: "/events",
        a: "/about",
      };
      const destination = mapping[event.key.toLowerCase()];
      const goto = gotoRef.current;
      if (destination && goto) {
        event.preventDefault();
        goto(destination);
      }
      clearChord();
    };

    window.addEventListener("keydown", handler);
    return () => {
      window.removeEventListener("keydown", handler);
      clearChord();
    };
  }, []);
}

function isEditableTarget(node: HTMLElement): boolean {
  if (node.isContentEditable) {
    return true;
  }
  const tag = node.tagName.toLowerCase();
  return tag === "input" || tag === "textarea" || tag === "select";
}
