import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { RitualRoot, RitualRootMissing, RitualRootResult } from "../api";
import {
  ROOT_TITLE_EMPTY, rootForbiddenSentence, rootMissingSentence, rootPlacementSentence, rootTakenNestedSentence, rootTakenTopSentence,
} from "../lib/ritualText";
import { RitualRootDialog } from "./RitualRootDialog";

const missing = (over: Partial<RitualRootMissing> = {}): RitualRootMissing => ({
  pageId: "653264152", spaceKey: "TEAM", canCreate: true, suggestedTitle: "PLAT Rituals", ...over,
});

const outcome = (over: Partial<RitualRoot> = {}): RitualRootResult => ({
  root: { outcome: "created", pageId: "9001", title: "PLAT Rituals", spaceKey: "TEAM", topLevel: true, ...over },
  sync: null,
  syncError: "",
});

type Props = ComponentProps<typeof RitualRootDialog>;

function renderDialog(over: Partial<Props> = {}) {
  const props: Props = {
    missing: missing(),
    create: vi.fn(async (_title: string, _adopt: boolean) => outcome()),
    onDone: vi.fn(),
    onOpenProfiles: vi.fn(),
    onClose: vi.fn(),
    ...over,
  };
  render(<RitualRootDialog {...props} />);
  return props;
}

describe("RitualRootDialog", () => {
  it("states the placement and creates the root with the suggested title", async () => {
    const props = renderDialog();
    expect(screen.getByRole("dialog", { name: "Rituals root page not found" })).toBeInTheDocument();
    expect(screen.getByText(rootMissingSentence("653264152"))).toBeInTheDocument();
    expect(screen.getByText(rootPlacementSentence("TEAM"))).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Page title" })).toHaveValue("PLAT Rituals");
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    expect(props.create).toHaveBeenCalledWith("PLAT Rituals", false);
    await waitFor(() => expect(props.onDone).toHaveBeenCalledTimes(1));
  });

  it("offers Profile settings beside Cancel and Create on the confirm step too", async () => {
    const props = renderDialog();
    await userEvent.click(screen.getByRole("button", { name: "Open Profile settings" }));
    expect(props.onClose).toHaveBeenCalled();
    expect(props.onOpenProfiles).toHaveBeenCalled();
  });

  it("sends an edited title trimmed, and refuses an empty one", async () => {
    const props = renderDialog();
    const box = screen.getByRole("textbox", { name: "Page title" });
    await userEvent.clear(box);
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    expect(screen.getByRole("alert")).toHaveTextContent(ROOT_TITLE_EMPTY);
    expect(props.create).not.toHaveBeenCalled();
    await userEvent.type(box, "  Team rituals  {Enter}");
    expect(props.create).toHaveBeenCalledWith("Team rituals", false);
  });

  it("offers Profile settings instead of a create when the probe says the token cannot create", async () => {
    const props = renderDialog({ missing: missing({ canCreate: false }) });
    expect(screen.getByText(rootForbiddenSentence("TEAM"))).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Create page and sync" })).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Open Profile settings" }));
    expect(props.onClose).toHaveBeenCalled();
    expect(props.onOpenProfiles).toHaveBeenCalled();
  });

  it("shows the forbidden sentence when the create itself is refused", async () => {
    const props = renderDialog({ create: vi.fn(async () => outcome({ outcome: "forbidden", pageId: "" })) });
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    expect(await screen.findByText(rootForbiddenSentence("TEAM"))).toBeInTheDocument();
    expect(props.onDone).not.toHaveBeenCalled();
  });

  it("asks a second time before adopting a top-level page with the same title", async () => {
    const create = vi.fn(async (_title: string, adopt: boolean) => outcome(adopt ? { outcome: "adopted", pageId: "77" } : { outcome: "titleTaken", pageId: "77" }));
    const props = renderDialog({ create });
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    expect(await screen.findByText(rootTakenTopSentence("PLAT Rituals", "TEAM"))).toBeInTheDocument();
    expect(props.onDone).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "Use this page and sync" }));
    expect(create).toHaveBeenLastCalledWith("PLAT Rituals", true);
    await waitFor(() => expect(props.onDone).toHaveBeenCalledTimes(1));
  });

  it("does not offer to adopt a same-title page below another page", async () => {
    renderDialog({ create: vi.fn(async () => outcome({ outcome: "titleTaken", pageId: "77", topLevel: false })) });
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    expect(await screen.findByText(rootTakenNestedSentence("PLAT Rituals", "TEAM"))).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Use this page and sync" })).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByRole("textbox", { name: "Page title" })).toBeInTheDocument();
  });

  it("refuses Escape while its call is in flight, and shows a failure with the title kept", async () => {
    let fail: (e: Error) => void = () => {};
    const props = renderDialog({ create: vi.fn(() => new Promise<RitualRootResult>((_resolve, reject) => { fail = reject; })) });
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    const pressed = screen.getByRole("button", { name: "Creating page…" });
    expect(pressed).toHaveAttribute("aria-disabled", "true");
    await userEvent.keyboard("{Escape}");
    expect(props.onClose).not.toHaveBeenCalled();
    await userEvent.click(pressed);
    expect(props.create).toHaveBeenCalledTimes(1);
    await act(async () => { fail(new Error("network down")); });
    expect(await screen.findByRole("alert")).toHaveTextContent("network down");
    const box = screen.getByRole("textbox", { name: "Page title" });
    expect(box).toHaveValue("PLAT Rituals");
    expect(box).toHaveFocus();
    expect(screen.getByRole("button", { name: "Create page and sync" })).not.toHaveAttribute("aria-disabled", "true");
  });

  it("keeps focus inside the dialog on the pressed button while the call runs", async () => {
    renderDialog({ create: vi.fn(() => new Promise<RitualRootResult>(() => {})) });
    await userEvent.type(screen.getByRole("textbox", { name: "Page title" }), "{Enter}");
    expect(screen.getByRole("button", { name: "Creating page…" })).toHaveFocus();
  });

  it("states a refused create as a status and moves focus to Open Profile settings", async () => {
    renderDialog({ create: vi.fn(async () => outcome({ outcome: "forbidden", pageId: "" })) });
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    expect(await screen.findByRole("status")).toHaveTextContent(rootForbiddenSentence("TEAM"));
    expect(screen.getByRole("button", { name: "Open Profile settings" })).toHaveFocus();
  });

  it("states a taken title as a status and moves focus to Use this page and sync", async () => {
    renderDialog({ create: vi.fn(async () => outcome({ outcome: "titleTaken", pageId: "77" })) });
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    expect(await screen.findByRole("status")).toHaveTextContent(rootTakenTopSentence("PLAT Rituals", "TEAM"));
    expect(screen.getByRole("button", { name: "Use this page and sync" })).toHaveFocus();
  });

  it("keeps the adopt option when the adopt call throws, so a retry resumes adoption", async () => {
    const create = vi.fn(async (_title: string, adopt: boolean) => {
      if (adopt) throw new Error("network down");
      return outcome({ outcome: "titleTaken", pageId: "77" });
    });
    renderDialog({ create });
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    await userEvent.click(await screen.findByRole("button", { name: "Use this page and sync" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("network down");
    const adopt = screen.getByRole("button", { name: "Use this page and sync" });
    expect(adopt).toHaveFocus();
    await userEvent.click(adopt);
    expect(create).toHaveBeenLastCalledWith("PLAT Rituals", true);
  });
});
