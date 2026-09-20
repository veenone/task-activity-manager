// Helpers for asserting the shared row lead (core's RowLead) from a render
// test. jsdom applies no stylesheet, so there are no pixels to read: what a
// test can see is the depth the row declares and what the lead reserves, and
// those are exactly what the stylesheet turns into the indent.

/** The depth the row's lead declares, NaN when the row drew no lead. */
export function leadDepth(row: HTMLElement): number {
  return Number(row.querySelector<HTMLElement>(".row-lead")?.dataset.rowDepth);
}

/** The text in the row's toggle slot, null when the row reserved no slot. */
export function leadSlotText(row: HTMLElement): string | null {
  return row.querySelector<HTMLElement>(".row-lead-slot")?.textContent ?? null;
}
