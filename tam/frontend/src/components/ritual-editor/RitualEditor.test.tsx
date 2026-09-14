import { createRef, useState } from "react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as api from "../../api";
import { READ_ONLY_SENTENCE, ENTRY_EXISTS, ENTRY_UNREADABLE } from "../../lib/ritualText";
import { RitualEditor } from "./RitualEditor";
import type { RitualEditorHandle } from "./RitualEditor";

vi.mock("../../api", async () => {
  const actual = await vi.importActual<typeof import("../../api")>("../../api");
  return { ...actual, SaveRitualBody: vi.fn(), StandupEntry: vi.fn(), RitualMacroIssues: vi.fn(), BrowserOpenURL: vi.fn() };
});

// jsdom has no layout; ProseMirror asks for rectangles when it scrolls a
// selection into view.
beforeAll(() => {
  Range.prototype.getBoundingClientRect = () => new DOMRect();
  Range.prototype.getClientRects = () => ({ length: 0, item: () => null, [Symbol.iterator]: [][Symbol.iterator] }) as unknown as DOMRectList;
  document.elementFromPoint = () => null;
});

function doc(over: Partial<api.RitualDocument> = {}): api.RitualDocument {
  return {
    profileId: "p1", boardId: 1, sprintId: 14, ritualType: "planning", title: "Sprint 14 · Planning",
    body: "<h2>Decisions</h2><ul><li>ship it</li></ul>", baseBody: "", pageId: "", version: 0,
    conflictBody: "", conflictVersion: 0, status: "local", updatedAt: "2026-09-14T09:00:00Z", syncedAt: "", ...over,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.SaveRitualBody).mockImplementation(async (_p, _b, _s, _t, body) => doc({ body, status: "local" }));
});

const pause = (ms: number) => act(() => new Promise((r) => setTimeout(r, ms)));

describe("RitualEditor", () => {
  it("never saves a page just because it was opened", async () => {
    render(<RitualEditor profileId="p1" doc={doc()} saveDelayMs={10} />);
    expect(await screen.findByText("ship it")).toBeInTheDocument();
    await pause(50);
    expect(api.SaveRitualBody).not.toHaveBeenCalled();
  });

  it("saves once after an edit pauses, and hands the saved row back", async () => {
    const ref = createRef<RitualEditorHandle>();
    const onSaved = vi.fn();
    render(<RitualEditor ref={ref} profileId="p1" doc={doc()} onSaved={onSaved} saveDelayMs={10} />);
    await screen.findByText("ship it");
    act(() => { ref.current!.editor!.commands.insertContentAt(ref.current!.editor!.state.doc.content.size, "<p>hello</p>"); });
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalledTimes(1));
    expect(vi.mocked(api.SaveRitualBody).mock.calls[0]).toEqual(["p1", 1, 14, "planning", "<h2>Decisions</h2><ul><li>ship it</li></ul><p>hello</p>"]);
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
  });

  it("flushes a pending edit immediately when asked", async () => {
    const ref = createRef<RitualEditorHandle>();
    render(<RitualEditor ref={ref} profileId="p1" doc={doc()} saveDelayMs={60_000} />);
    await screen.findByText("ship it");
    act(() => { ref.current!.editor!.commands.insertContentAt(ref.current!.editor!.state.doc.content.size, "<p>now</p>"); });
    await act(() => ref.current!.flush());
    expect(api.SaveRitualBody).toHaveBeenCalledTimes(1);
  });

  it("saves a mouse-only task checkbox tick", async () => {
    render(<RitualEditor profileId="p1" saveDelayMs={10}
      doc={doc({ body: "<ac:task-list><ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body><p>x</p></ac:task-body></ac:task></ac:task-list>" })} />);
    await screen.findByText("x");
    const checkbox = document.querySelector('ul[data-type="taskList"] li input[type="checkbox"]') as HTMLInputElement;
    expect(checkbox).toBeTruthy();
    fireEvent.click(checkbox);
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalled());
    const body = vi.mocked(api.SaveRitualBody).mock.calls[0][4];
    expect(body).toContain("<ac:task-status>complete</ac:task-status>");
  });

  it("survives its own save: the editor instance is not rebuilt when the saved row comes back", async () => {
    const onSavedSpy = vi.fn();
    function Harness({ handleRef }: { handleRef: React.RefObject<RitualEditorHandle | null> }) {
      const [d, setD] = useState<api.RitualDocument>(doc());
      return <RitualEditor ref={handleRef} profileId="p1" doc={d}
        onSaved={(saved) => { onSavedSpy(saved); setD(saved); }} saveDelayMs={10} />;
    }
    const ref = createRef<RitualEditorHandle>();
    render(<Harness handleRef={ref} />);
    await screen.findByText("ship it");
    const before = ref.current!.editor;
    act(() => { ref.current!.editor!.commands.insertContentAt(ref.current!.editor!.state.doc.content.size, "<p>more</p>"); });
    // Wait for the save's effect on the parent (onSaved, which also updates
    // Harness's own doc state) to have actually landed before checking
    // identity with a plain expect: a waitFor around the identity check
    // itself would pass on its very first poll, before the save or the
    // re-render it triggers have happened at all, proving nothing.
    await waitFor(() => expect(onSavedSpy).toHaveBeenCalledTimes(1));
    expect(ref.current!.editor).toBe(before);
  });

  it("saves again if an undo lands back on the stored text while a save is in flight", async () => {
    const ref = createRef<RitualEditorHandle>();
    let resolveSave: ((saved: api.RitualDocument) => void) | null = null;
    vi.mocked(api.SaveRitualBody).mockImplementationOnce(() => new Promise((resolve) => { resolveSave = resolve; }));
    render(<RitualEditor ref={ref} profileId="p1" doc={doc()} saveDelayMs={10} />);
    await screen.findByText("ship it");
    const originalBody = doc().body;
    act(() => { ref.current!.editor!.commands.insertContentAt(ref.current!.editor!.state.doc.content.size, "<p>hello</p>"); });
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalledTimes(1));
    // Undo back to the original text while that first save is still in flight.
    act(() => { ref.current!.editor!.commands.undo(); });
    await act(async () => { resolveSave!(doc({ body: "<h2>Decisions</h2><ul><li>ship it</li></ul><p>hello</p>", status: "local" })); });
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.SaveRitualBody).mock.calls[1][4]).toBe(originalBody);
  });

  it("does not resend the undone text if the in-flight save fails", async () => {
    const ref = createRef<RitualEditorHandle>();
    let rejectSave: ((e: unknown) => void) | null = null;
    vi.mocked(api.SaveRitualBody).mockImplementationOnce(() => new Promise((_resolve, reject) => { rejectSave = reject; }));
    render(<RitualEditor ref={ref} profileId="p1" doc={doc()} saveDelayMs={10} />);
    await screen.findByText("ship it");
    act(() => { ref.current!.editor!.commands.insertContentAt(ref.current!.editor!.state.doc.content.size, "<p>hello</p>"); });
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalledTimes(1));
    act(() => { ref.current!.editor!.commands.undo(); });
    await act(async () => { rejectSave!(new Error("network down")); });
    // The undo already differs from the stored text (still the original,
    // since the failed save never persisted anything), so it retries with
    // the original body; confirm that retry actually happens, not just that
    // it carries the right text, or this test would pass just as well if
    // nothing were sent at all.
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalledTimes(2));
    // The first call is the failed attempt itself, and legitimately carries
    // "hello": what must never happen is a later call resending it after
    // the user undid it back out.
    for (const call of vi.mocked(api.SaveRitualBody).mock.calls.slice(1)) {
      expect(call[4]).not.toContain("hello");
    }
  });

  it("opens a page it cannot read as read only, with the page link", async () => {
    render(<RitualEditor profileId="p1" doc={doc({ body: "<p>&bogus;</p>" })} pageUrl="https://c.example.com/pages/viewpage.action?pageId=42" />);
    expect(screen.getByText(READ_ONLY_SENTENCE)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Open in Confluence" }));
    expect(api.BrowserOpenURL).toHaveBeenCalledWith("https://c.example.com/pages/viewpage.action?pageId=42");
    expect(document.querySelector(".ProseMirror")).toBeNull();
  });

  it("draws content it does not model as a locked block", async () => {
    render(<RitualEditor profileId="p1" doc={doc({ body: '<ac:structured-macro ac:name="panel"><ac:rich-text-body><p>x</p></ac:rich-text-body></ac:structured-macro>' })} />);
    expect(await screen.findByText("Confluence: panel")).toBeInTheDocument();
  });

  it("adds today's standup entry under the log once, and refuses a second", async () => {
    vi.mocked(api.StandupEntry).mockResolvedValue("<h3>Tue 15 Sep 2026</h3><p><strong>Yesterday</strong></p><ul><li></li></ul>");
    const ref = createRef<RitualEditorHandle>();
    render(<RitualEditor ref={ref} profileId="p1" saveDelayMs={10}
      doc={doc({ ritualType: "standup", body: "<h2>Daily log</h2><h3>Mon 14 Sep 2026</h3><p>y</p>" })} />);
    await screen.findByText("Mon 14 Sep 2026");
    await userEvent.click(screen.getByRole("button", { name: "Add today's entry" }));
    await waitFor(() => expect(ref.current!.editor!.state.doc.child(1).textContent).toBe("Tue 15 Sep 2026"));
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalled());
    await userEvent.click(screen.getByRole("button", { name: "Add today's entry" }));
    expect(await screen.findByText(ENTRY_EXISTS)).toBeInTheDocument();
  });

  it("shows a notice and does not crash when the standup entry fails to read", async () => {
    vi.mocked(api.StandupEntry).mockRejectedValue(new Error("network down"));
    render(<RitualEditor profileId="p1" saveDelayMs={10}
      doc={doc({ ritualType: "standup", body: "<h2>Daily log</h2><h3>Mon 14 Sep 2026</h3><p>y</p>" })} />);
    await screen.findByText("Mon 14 Sep 2026");
    await userEvent.click(screen.getByRole("button", { name: "Add today's entry" }));
    expect(await screen.findByText("network down")).toBeInTheDocument();
  });

  it("shows a notice when the standup entry fragment cannot be parsed", async () => {
    vi.mocked(api.StandupEntry).mockResolvedValue("<p>&bogus;</p>");
    render(<RitualEditor profileId="p1" saveDelayMs={10}
      doc={doc({ ritualType: "standup", body: "<h2>Daily log</h2><h3>Mon 14 Sep 2026</h3><p>y</p>" })} />);
    await screen.findByText("Mon 14 Sep 2026");
    await userEvent.click(screen.getByRole("button", { name: "Add today's entry" }));
    expect(await screen.findByText(ENTRY_UNREADABLE)).toBeInTheDocument();
  });

  it("offers Add today's entry only on the standup", async () => {
    render(<RitualEditor profileId="p1" doc={doc()} />);
    await screen.findByText("ship it");
    expect(screen.queryByRole("button", { name: "Add today's entry" })).toBeNull();
  });
});
