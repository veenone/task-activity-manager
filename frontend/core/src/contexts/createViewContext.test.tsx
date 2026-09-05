import React from "react";
import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { createViewContext } from "./createViewContext";

const { ViewProvider, useView } = createViewContext<"backlog" | "boards">("backlog");

function wrapper({ children }: { children: React.ReactNode }) {
  return <ViewProvider>{children}</ViewProvider>;
}

describe("createViewContext", () => {
  it("starts on the initial view and switches", () => {
    const { result } = renderHook(() => useView(), { wrapper });
    expect(result.current.view).toBe("backlog");
    act(() => result.current.setView("boards"));
    expect(result.current.view).toBe("boards");
  });

  it("throws outside its provider", () => {
    expect(() => renderHook(() => useView())).toThrow(
      "useView must be used within a ViewProvider",
    );
  });
});
