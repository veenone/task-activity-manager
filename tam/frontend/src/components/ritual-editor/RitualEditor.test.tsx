import { createRef, useState } from "react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as api from "../../api";
import { READ_ONLY_SENTENCE, ENTRY_EXISTS, ENTRY_UNREADABLE, LINK_REFUSED } from "../../lib/ritualText";
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
    expect(vi.mocked(api.SaveRitualBody).mock.calls[0]).toEqual(["p1", 1, 14, "planning", "<h2>Decisions</h2><ul><li>ship it</li></ul><p>hello</p>", 0, ""]);
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
  });

  // The save names the version and page id the editor was opened on, so the
  // store can refuse text typed over a page a Sync has since replaced.
  it("saves against the version and page id it was opened on, and shows a refusal", async () => {
    const refused = "This page changed while you were editing (a Sync brought in a newer version). Copy your text, reopen the page, and apply it again.";
    vi.mocked(api.SaveRitualBody).mockRejectedValue(new Error(refused));
    const ref = createRef<RitualEditorHandle>();
    render(<RitualEditor ref={ref} profileId="p1" doc={doc({ pageId: "42", version: 3, status: "synced" })} saveDelayMs={10} />);
    await screen.findByText("ship it");
    act(() => { ref.current!.editor!.commands.insertContentAt(ref.current!.editor!.state.doc.content.size, "<p>late</p>"); });
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalledTimes(1));
    expect(vi.mocked(api.SaveRitualBody).mock.calls[0].slice(5)).toEqual([3, "42"]);
    expect(await screen.findByText(`Not saved: ${refused}`)).toBeInTheDocument();
    // The text stays pending: the next trigger sends it again.
    await act(() => ref.current!.flush());
    expect(api.SaveRitualBody).toHaveBeenCalledTimes(2);
    expect(vi.mocked(api.SaveRitualBody).mock.calls[1][4]).toContain("late");
  });

  // A Sync must not meet keystrokes typed after its flush, and must not cost
  // the caret or undo history either, so the lock is setEditable on the same
  // instance rather than a rebuild. The toolbar stays drawn, disabled, so the
  // page does not jump when a Sync starts.
  it("is not editable while locked, with its toolbar disabled not hidden, and editable again on the same editor", async () => {
    const ref = createRef<RitualEditorHandle>();
    const { rerender } = render(<RitualEditor ref={ref} profileId="p1" doc={doc()} locked />);
    await screen.findByText("ship it");
    const before = ref.current!.editor!;
    await waitFor(() => expect(before.isEditable).toBe(false));
    const locked = screen.getByRole("toolbar", { name: "Formatting" });
    for (const button of within(locked).getAllByRole("button")) expect(button).toBeDisabled();
    rerender(<RitualEditor ref={ref} profileId="p1" doc={doc()} locked={false} />);
    await waitFor(() => expect(ref.current!.editor!.isEditable).toBe(true));
    expect(ref.current!.editor).toBe(before);
    expect(within(screen.getByRole("toolbar", { name: "Formatting" })).getByRole("button", { name: "Bold" })).toBeEnabled();
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

  it("a failed save whose undo already matches what is stored does not resend it", async () => {
    const ref = createRef<RitualEditorHandle>();
    let rejectSave: ((e: unknown) => void) | null = null;
    vi.mocked(api.SaveRitualBody).mockImplementationOnce(() => new Promise((_resolve, reject) => { rejectSave = reject; }));
    render(<RitualEditor ref={ref} profileId="p1" doc={doc()} saveDelayMs={10} />);
    await screen.findByText("ship it");
    act(() => { ref.current!.editor!.commands.insertContentAt(ref.current!.editor!.state.doc.content.size, "<p>hello</p>"); });
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalledTimes(1));
    // Undo back to exactly what is stored: the failed save never persisted
    // anything, so the store still holds the original text, and the editor
    // now reads back to that same text too.
    act(() => { ref.current!.editor!.commands.undo(); });
    await act(async () => { rejectSave!(new Error("network down")); });
    // Nothing left to save: a pending equal to what is already stored must
    // not queue a save that would change no content, so no second call ever
    // happens, and the one call that did happen (the failed attempt itself)
    // never gets resent. Give a stray timer a real chance to fire before
    // concluding nothing else was queued.
    await pause(50);
    expect(api.SaveRitualBody).toHaveBeenCalledTimes(1);
    expect(vi.mocked(api.SaveRitualBody).mock.calls[0][4]).toContain("hello");
  });

  it("a save that keeps failing waits for the next edit instead of retrying itself", async () => {
    vi.mocked(api.SaveRitualBody).mockRejectedValue(new Error("no planning ritual is stored for sprint 14"));
    const ref = createRef<RitualEditorHandle>();
    render(<RitualEditor ref={ref} profileId="p1" doc={doc()} saveDelayMs={10} />);
    await screen.findByText("ship it");
    act(() => { ref.current!.editor!.commands.insertContentAt(ref.current!.editor!.state.doc.content.size, "<p>hello</p>"); });
    // Well past the debounce: a save that retried itself on failure would
    // have called SaveRitualBody many times over by now (measured at 20
    // calls in 300 ms against a mock rejecting after 5 ms in the review
    // that found this).
    await pause(150);
    expect(api.SaveRitualBody).toHaveBeenCalledTimes(1);
    // A further edit is one of the real triggers a failed save waits for.
    act(() => { ref.current!.editor!.commands.insertContentAt(ref.current!.editor!.state.doc.content.size, "<p>world</p>"); });
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalledTimes(2));
  });

  it("a flush after a failed save retries once", async () => {
    vi.mocked(api.SaveRitualBody).mockRejectedValue(new Error("no planning ritual is stored for sprint 14"));
    const ref = createRef<RitualEditorHandle>();
    render(<RitualEditor ref={ref} profileId="p1" doc={doc()} saveDelayMs={10} />);
    await screen.findByText("ship it");
    act(() => { ref.current!.editor!.commands.insertContentAt(ref.current!.editor!.state.doc.content.size, "<p>hello</p>"); });
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalledTimes(1));
    await act(() => ref.current!.flush());
    expect(api.SaveRitualBody).toHaveBeenCalledTimes(2);
  });

  it("opens a page it cannot read as read only, with the page link", async () => {
    render(<RitualEditor profileId="p1" doc={doc({ body: "<p>&bogus;</p>" })} pageUrl="https://c.example.com/pages/viewpage.action?pageId=42" />);
    expect(screen.getByText(READ_ONLY_SENTENCE)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Open in Confluence" }));
    expect(api.BrowserOpenURL).toHaveBeenCalledWith("https://c.example.com/pages/viewpage.action?pageId=42");
    expect(document.querySelector(".ProseMirror")).toBeNull();
  });

  // A link followed inside the WebView navigates the whole app away from TAM.
  it("opens an http link from a read-only page in the browser, never in the window", async () => {
    render(<RitualEditor profileId="p1" doc={doc({ body: `<p>&bogus;</p><p><a href="https://example.com/notes">notes</a></p>` })} />);
    const link = screen.getByRole("link", { name: "notes" });
    expect(fireEvent.click(link)).toBe(false);
    expect(api.BrowserOpenURL).toHaveBeenCalledWith("https://example.com/notes");
  });

  it("follows a javascript: link nowhere", async () => {
    // Read-only fallback: the sanitizer has already taken the href away.
    const { unmount } = render(<RitualEditor profileId="p1" doc={doc({ body: `<p>&bogus;</p><p><a href="javascript:alert(1)">bad</a></p>` })} />);
    fireEvent.click(screen.getByText("bad"));
    unmount();
    // Editor: the click handler itself refuses anything but http and https.
    render(<RitualEditor profileId="p1" readOnly doc={doc({ body: `<p><a href="javascript:alert(1)">worse</a> <a href="mailto:a@example.com">mail</a></p>` })} />);
    for (const text of ["worse", "mail"]) {
      const anchor = (await screen.findByText(text)).closest("a")!;
      anchor.setAttribute("href", text === "worse" ? "java\tscript:alert(1)" : "mailto:a@example.com");
      expect(fireEvent.click(anchor)).toBe(false);
    }
    expect(api.BrowserOpenURL).not.toHaveBeenCalled();
  });

  it("opens a link in the read-only editor (View theirs) in the browser", async () => {
    render(<RitualEditor profileId="p1" readOnly doc={doc({ body: `<p><a href="https://example.com/theirs">theirs</a></p>` })} />);
    const link = await screen.findByRole("link", { name: "theirs" });
    expect(fireEvent.click(link)).toBe(false);
    expect(api.BrowserOpenURL).toHaveBeenCalledWith("https://example.com/theirs");
  });

  it("keeps a plain click on a link in the editable editor in place, and opens it on Ctrl+click", async () => {
    render(<RitualEditor profileId="p1" doc={doc({ body: `<p><a href="https://example.com/mine">mine</a></p>` })} />);
    const link = await screen.findByRole("link", { name: "mine" });
    expect(fireEvent.click(link)).toBe(false);
    expect(api.BrowserOpenURL).not.toHaveBeenCalled();
    expect(fireEvent.click(link, { ctrlKey: true })).toBe(false);
    expect(api.BrowserOpenURL).toHaveBeenCalledWith("https://example.com/mine");
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

  it("offers the formatting groups this editor supports", async () => {
    render(<RitualEditor profileId="p1" doc={doc()} />);
    await screen.findByText("ship it");
    const bar = screen.getByRole("toolbar", { name: "Formatting" });
    expect(within(bar).getAllByRole("group").map((g) => g.getAttribute("aria-label"))).toEqual(["Text", "Headings", "Lists", "Insert", "History"]);
    for (const name of ["Bold", "Italic", "Underline", "Strikethrough", "Heading 2", "Heading 3", "Bullet list", "Numbered list", "Task list", "Insert table", "Link", "Undo", "Redo"]) {
      expect(within(bar).getByRole("button", { name })).toBeInTheDocument();
    }
    // A page just opened has nothing to undo.
    expect(within(bar).getByRole("button", { name: "Undo" })).toHaveAttribute("aria-disabled", "true");
  });

  it("puts Add today's entry in its own Standup group", async () => {
    render(<RitualEditor profileId="p1" doc={doc({ ritualType: "standup", body: "<h2>Daily log</h2><h3>Mon 14 Sep 2026</h3><p>y</p>" })} />);
    await screen.findByText("Mon 14 Sep 2026");
    const group = screen.getByRole("group", { name: "Standup" });
    expect(within(group).getByRole("button", { name: "Add today's entry" })).toHaveTextContent("+ Add today's entry");
  });

  it("draws no toolbar on a read-only page", async () => {
    render(<RitualEditor profileId="p1" readOnly doc={doc()} />);
    await screen.findByText("ship it");
    expect(screen.queryByRole("toolbar", { name: "Formatting" })).toBeNull();
  });

  it("bolds the selection from the toolbar, shows it pressed, and saves it", async () => {
    const ref = createRef<RitualEditorHandle>();
    render(<RitualEditor ref={ref} profileId="p1" doc={doc()} saveDelayMs={10} />);
    await screen.findByText("ship it");
    act(() => { ref.current!.editor!.commands.selectAll(); });
    const bold = screen.getByRole("button", { name: "Bold" });
    await userEvent.click(bold);
    await waitFor(() => expect(bold).toHaveAttribute("aria-pressed", "true"));
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalled());
    expect(vi.mocked(api.SaveRitualBody).mock.calls.at(-1)![4]).toContain("<strong>");
  });

  it("refuses a link scheme it does not allow and says why, then links an https address", async () => {
    const ref = createRef<RitualEditorHandle>();
    render(<RitualEditor ref={ref} profileId="p1" doc={doc()} />);
    await screen.findByText("ship it");
    act(() => { ref.current!.editor!.commands.selectAll(); });
    await userEvent.click(screen.getByRole("button", { name: "Link" }));
    const input = screen.getByRole("textbox", { name: "Link address" });
    await userEvent.type(input, "javascript:alert(1){Enter}");
    expect(await screen.findByText(LINK_REFUSED)).toBeInTheDocument();
    expect(ref.current!.editor!.getHTML()).not.toContain("javascript");
    await userEvent.clear(input);
    await userEvent.type(input, "https://example.com/notes{Enter}");
    await waitFor(() => expect(screen.queryByRole("textbox", { name: "Link address" })).toBeNull());
    expect(ref.current!.editor!.getHTML()).toContain('href="https://example.com/notes"');
  });
});
