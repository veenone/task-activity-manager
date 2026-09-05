import React from "react";
import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { createModalContext } from "./createModalContext";

const { ModalProvider, useModal } = createModalContext<"profiles" | "about">();

function wrapper({ children }: { children: React.ReactNode }) {
  return <ModalProvider>{children}</ModalProvider>;
}

describe("createModalContext", () => {
  it("opens one modal at a time and closes it", () => {
    const { result } = renderHook(() => useModal(), { wrapper });
    expect(result.current.current).toBeNull();
    act(() => result.current.openModal("profiles"));
    expect(result.current.isOpen("profiles")).toBe(true);
    act(() => result.current.openModal("about"));
    expect(result.current.isOpen("profiles")).toBe(false);
    expect(result.current.current).toBe("about");
    act(() => result.current.closeModal());
    expect(result.current.current).toBeNull();
  });

  it("throws outside its provider", () => {
    expect(() => renderHook(() => useModal())).toThrow(
      "useModal must be used within a ModalProvider",
    );
  });
});
