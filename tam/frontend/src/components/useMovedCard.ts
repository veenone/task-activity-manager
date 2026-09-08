import { useEffect, useRef, useState } from "react";
import { announce } from "@agile-suite/core";
import type { BoardView } from "../api";
import { findCard, posId } from "../lib/boardCells";
import { landedMessage } from "../lib/cardMove";
import type { MoveIntent } from "../lib/cardMove";
import { MOVED_FLASH_MS } from "./EpicTree";

// useMovedCard follows a moved card rather than the slot it left. posId is
// lane-col-index, so the focused id names a place on the board and not a
// card: after a move that id holds a different card, and a second Ctrl and
// Right would move that one instead. So the key is tracked, and once the
// board query has settled the card is found wherever it landed, focus goes
// there, the move announces where "there" is, and the card flashes, which
// is what EpicTree already does for a reparented row.
//
// It returns the key to flash, "" for none.
export function useMovedCard(
  view: BoardView | undefined,
  settled: boolean,
  intent: MoveIntent | null,
  onLanded: (id: string) => void,
): string {
  const [flashKey, setFlashKey] = useState("");
  // The intent already answered. A stamp rather than a key, so two moves
  // of the same card are two announcements.
  const said = useRef(0);
  const landed = useRef(onLanded);
  landed.current = onLanded;

  useEffect(() => {
    if (!intent || !view || !settled || said.current === intent.at) return;
    said.current = intent.at;
    const p = findCard(view, intent.key);
    announce(landedMessage(intent, view, p));
    // A card that left this board, which a sprint move does, has nowhere
    // to be focused or flashed. The announcement above still names where
    // it went, which is the only report of it the user gets.
    if (!p) return;
    landed.current(posId(p));
    setFlashKey(intent.key);
  }, [view, settled, intent]);

  useEffect(() => {
    if (!flashKey) return;
    const t = setTimeout(() => setFlashKey(""), MOVED_FLASH_MS);
    return () => clearTimeout(t);
  }, [flashKey]);

  return flashKey;
}
