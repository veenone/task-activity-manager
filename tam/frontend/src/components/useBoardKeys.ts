import { useEffect, useRef, useState } from "react";
import type { FocusEvent, KeyboardEvent } from "react";
import type { BoardView, Issue } from "../api";
import { cardAtPos, clampFocus, findCard, moveFocus, parsePos, posId } from "../lib/boardCells";
import { CARD_MENU_CLASS } from "./CardMoveMenu";
import type { BoardMoves } from "./useBoardMoves";
import type { BoardSelection } from "./useBoardSelection";
import { useMovedCard } from "./useMovedCard";

interface Args {
  // data is the board as drawn, filter and all: the cells the keyboard
  // walks have to be the cells the reader sees.
  data: BoardView | undefined;
  // fetching holds the move announcement back until the board has redrawn,
  // so a card is reported where it landed rather than where it was sent.
  fetching: boolean;
  moves: BoardMoves;
  selection: BoardSelection;
  selectedKey: string;
  focusId: string;
  setFocusId: (id: string) => void;
  // onOpen is Enter: the one key that still opens the detail panel.
  onOpen: (issue: Issue, id: string) => void;
}

// useBoardKeys is the board's focus and its keyboard, in one place: which
// card is focusable, what each key does to it, and the two pieces of state
// that only exist because a move unmounts the card focus was on.
//
// The key map is crowded, and every branch below is load bearing. Enter
// opens the panel; Space alone checks the card, which is the gesture that
// used to duplicate Enter; Control with an arrow is a move; Shift with an
// arrow extends the checked run; the menu key opens the card's own menu.
// Control with Space never arrives, because the Space branch returns before
// the Control one.
export function useBoardKeys({
  data, fetching, moves, selection, selectedKey, focusId, setFocusId, onOpen,
}: Args) {
  // menuOpenedOn is the card whose move menu the keyboard has just opened.
  // The panel does not exist until React has flushed the trigger's own
  // click, so the focus that follows has to wait for the render rather than
  // run in the key press that asked for it.
  const [menuOpenedOn, setMenuOpenedOn] = useState("");
  const bodyRef = useRef<HTMLDivElement>(null);
  // Whether focus was inside the board when the last move was made. A move
  // across columns unmounts the focused card and mounts a new one in the
  // other cell, so by the time the board has redrawn document.activeElement
  // is already the page body: asking then would answer no every time, and
  // focus would be left behind on the first Ctrl and an arrow.
  const hadFocus = useRef(false);

  function cardEl(id: string): HTMLElement | null | undefined {
    return bodyRef.current?.querySelector<HTMLElement>(`[data-board-pos="${id}"]`);
  }

  const flashKey = useMovedCard(data, !fetching, moves.intent, (id) => {
    setFocusId(id);
    // Focus follows the card only when the board already had it: a drag
    // made with the mouse must not pull focus out of wherever the user put
    // it.
    if (hadFocus.current) cardEl(id)?.focus();
  });

  // The card's menu is opened by pressing the trigger the mouse presses,
  // and focus goes into the panel that press produced, once React has drawn
  // it.
  useEffect(() => {
    if (!menuOpenedOn) return;
    const card = bodyRef.current?.querySelector<HTMLElement>(`[data-board-pos="${menuOpenedOn}"]`);
    card?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
    setMenuOpenedOn("");
  }, [menuOpenedOn]);

  // Exactly one card is focusable, in every state: the one focus is on when
  // it survived the last change, else the selected card, else the first card
  // on the board.
  const focus = data ? clampFocus(data, focusId, findCard(data, selectedKey)) : "";

  // openMenu is the keyboard's way into the card's move menu. The focus that
  // follows is left to the effect above: the panel is a state change React
  // flushes when this handler returns, so querying for a menu item here
  // would search a board that still has no panel in it.
  function openMenu(id: string) {
    const trigger = cardEl(id)?.querySelector<HTMLElement>(`.${CARD_MENU_CLASS}`);
    if (!trigger) return;
    trigger.click();
    setMenuOpenedOn(id);
  }

  // rememberFocus keeps hadFocus true across the unmount a move makes: a
  // card leaving the DOM takes focus to the body with no related target,
  // and that is the case the flag exists for. Focus genuinely leaving the
  // board, which names where it went, is what clears it.
  function rememberFocus(e: FocusEvent<HTMLDivElement>) {
    if (e.type === "focus") {
      hadFocus.current = true;
      return;
    }
    if (e.relatedTarget && !bodyRef.current?.contains(e.relatedTarget)) hadFocus.current = false;
  }

  function onKeyDown(e: KeyboardEvent, id: string) {
    if (!data) return;
    const p = parsePos(id);
    if (!p) return;
    const issue = cardAtPos(data, p);
    if (e.key === "Enter") {
      e.preventDefault();
      if (issue) onOpen(issue, id);
      return;
    }
    // Space belongs to the board in every combination, not only on its own:
    // the board is the scroller, so a Space that falls through scrolls the
    // cards out from under the card focus is on. It is suppressed either
    // way, as 3b's branch suppressed it, and only the unmodified one checks.
    if (e.key === " ") {
      e.preventDefault();
      if (issue && !e.ctrlKey && !e.shiftKey && !e.altKey) selection.check(issue.key);
      return;
    }
    if (e.key === "ContextMenu" || (e.shiftKey && e.key === "F10")) {
      e.preventDefault();
      openMenu(id);
      return;
    }
    if (e.ctrlKey && issue && moves.moveByKeyboard(p, issue, e.key)) {
      e.preventDefault();
      return;
    }
    const next = moveFocus(data, p, e.key);
    if (!next) return;
    e.preventDefault();
    // Shift with an arrow is the keyboard's shift-click: focus moves and the
    // run from the anchor grows with it.
    if (e.shiftKey) {
      const landed = cardAtPos(data, next);
      if (landed) selection.extendTo(landed.key);
    }
    const nextId = posId(next);
    setFocusId(nextId);
    cardEl(nextId)?.focus();
  }

  return { bodyRef, focus, flashKey, onKeyDown, rememberFocus };
}
