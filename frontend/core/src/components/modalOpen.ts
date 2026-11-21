// isModalOpen says whether a dialog built on Modal is on screen. The view
// shortcuts ask it first: a native menu accelerator fires whatever has focus,
// and switching views under a dialog unmounts the draft it holds (#528).
export function isModalOpen(): boolean {
  return document.querySelector('[aria-modal="true"]') !== null;
}
