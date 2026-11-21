import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { Modal } from "./Modal";
import { isModalOpen } from "./modalOpen";

// The view shortcuts ask this before switching, so a dialog's draft is not
// unmounted under it (#528).
describe("isModalOpen", () => {
  it("is false with no dialog on screen and true while one is open", () => {
    expect(isModalOpen()).toBe(false);
    const { unmount } = render(
      <Modal label="New issue" onClose={() => {}}>
        draft
      </Modal>,
    );
    expect(isModalOpen()).toBe(true);
    unmount();
    expect(isModalOpen()).toBe(false);
  });
});
