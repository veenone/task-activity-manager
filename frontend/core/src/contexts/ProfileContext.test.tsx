import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import { ProfileProvider, useProfile } from "./ProfileContext";
import type { ProfileBackend } from "./ProfileContext";

interface P { id: string; name: string }
interface S { defaultProfileId: string; theme: string }

const backend: ProfileBackend<P, S> = {
  listProfiles: vi.fn(),
  getSettings: vi.fn(),
  setTheme: vi.fn(),
  setDefaultProfile: vi.fn(),
};

function wrapper({ children }: { children: React.ReactNode }) {
  return <ProfileProvider backend={backend}>{children}</ProfileProvider>;
}

beforeEach(() => {
  vi.mocked(backend.listProfiles).mockResolvedValue([
    { id: "a", name: "A" },
    { id: "b", name: "B" },
  ]);
  vi.mocked(backend.getSettings).mockResolvedValue({
    defaultProfileId: "b",
    theme: "dark",
  });
  vi.mocked(backend.setTheme).mockResolvedValue();
  vi.mocked(backend.setDefaultProfile).mockResolvedValue();
  document.documentElement.dataset.theme = "";
});

describe("ProfileProvider", () => {
  it("loads profiles, picks the default, and applies the theme", async () => {
    const { result } = renderHook(() => useProfile<P, S>(), { wrapper });
    await act(async () => {
      await result.current.reload();
    });
    expect(result.current.profiles.map((p) => p.id)).toEqual(["a", "b"]);
    expect(result.current.activeId).toBe("b");
    expect(result.current.activeProfile?.name).toBe("B");
    expect(document.documentElement.dataset.theme).toBe("dark");
  });

  it("falls back to the first profile when the default is gone", async () => {
    vi.mocked(backend.getSettings).mockResolvedValue({
      defaultProfileId: "zzz",
      theme: "light",
    });
    const { result } = renderHook(() => useProfile<P, S>(), { wrapper });
    await act(async () => {
      await result.current.reload();
    });
    expect(result.current.activeId).toBe("a");
  });

  it("setDefault toggles and persists", async () => {
    const { result } = renderHook(() => useProfile<P, S>(), { wrapper });
    await act(async () => {
      await result.current.reload();
    });
    await act(async () => {
      await result.current.setDefault("b");
    });
    expect(backend.setDefaultProfile).toHaveBeenCalledWith("");
    expect(result.current.defaultProfileId).toBe("");
  });

  it("setTheme applies immediately and persists", async () => {
    const { result } = renderHook(() => useProfile<P, S>(), { wrapper });
    await act(async () => {
      await result.current.setTheme("light");
    });
    await waitFor(() => expect(backend.setTheme).toHaveBeenCalledWith("light"));
    expect(document.documentElement.dataset.theme).toBe("light");
  });
});
