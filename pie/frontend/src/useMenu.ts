import { useEffect, useRef } from "react";
import { EventsOn } from "../wailsjs/runtime/runtime";

// A handler takes the event's one payload, if it has one.
export type MenuHandlers = Record<string, (arg: never) => void>;

// useMenu listens for the native menu's events, named "menu:<name>" in
// pie/menu.go. The listeners are registered once and always call the latest
// handlers, so a handler can read the render's state without re-registering.
// While a dialog is open every event is dropped (#547): the controls the menu
// stands for sit behind the dialog, and a view switch would unmount a dialog
// that a view owns.
export function useMenu(handlers: MenuHandlers) {
  const latest = useRef(handlers);
  latest.current = handlers;
  const names = Object.keys(handlers).join(",");
  useEffect(() => {
    const offs = names.split(",").map((name) =>
      EventsOn(`menu:${name}`, (arg: unknown) => {
        if (document.querySelector('[aria-modal="true"]')) return;
        latest.current[name]?.(arg as never);
      }),
    );
    return () => offs.forEach((off) => off());
  }, [names]);
}
