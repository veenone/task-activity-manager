import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { RitualRoot } from "../api";
import { ReportRootDialog } from "./ReportRootDialog";

const root = (over: Partial<RitualRoot> = {}): RitualRoot => ({
  outcome: "created", pageId: "9100", title: "PLAT Reports", spaceKey: "REPORTS", topLevel: true, ...over,
});

type Props = ComponentProps<typeof ReportRootDialog>;

function renderDialog(over: Partial<Props> = {}) {
  const props: Props = {
    spaceKey: "REPORTS",
    suggestedTitle: "PLAT Reports",
    create: vi.fn(async (_title: string, _adopt: boolean) => root()),
    onChosen: vi.fn(),
    onClose: vi.fn(),
    ...over,
  };
  render(<ReportRootDialog {...props} />);
  return props;
}

describe("ReportRootDialog", () => {
  it("creates the page with the suggested title and hands the id back", async () => {
    const props = renderDialog();
    expect(screen.getByRole("dialog", { name: "Choose a reports root page" })).toBeInTheDocument();
    expect(screen.getByLabelText("Page title")).toHaveValue("PLAT Reports");

    await userEvent.click(screen.getByRole("button", { name: "Create page" }));
    await waitFor(() => expect(props.create).toHaveBeenCalledWith("PLAT Reports", false));
    expect(props.onChosen).toHaveBeenCalledWith(root());
  });

  it("offers to use a top level page whose title is already taken, and adopts it", async () => {
    const create = vi
      .fn<(title: string, adopt: boolean) => Promise<RitualRoot>>()
      .mockResolvedValueOnce(root({ outcome: "titleTaken", pageId: "7", topLevel: true }))
      .mockResolvedValueOnce(root({ outcome: "adopted", pageId: "7" }));
    const props = renderDialog({ create });

    await userEvent.click(screen.getByRole("button", { name: "Create page" }));
    const use = await screen.findByRole("button", { name: "Use this page" });
    expect(props.onChosen).not.toHaveBeenCalled();

    await userEvent.click(use);
    await waitFor(() => expect(create).toHaveBeenLastCalledWith("PLAT Reports", true));
    expect(props.onChosen).toHaveBeenCalledWith(root({ outcome: "adopted", pageId: "7" }));
  });

  it("says when the token may not create pages there, and hands nothing back", async () => {
    const props = renderDialog({ create: vi.fn(async () => root({ outcome: "forbidden", pageId: "" })) });
    await userEvent.click(screen.getByRole("button", { name: "Create page" }));
    expect(await screen.findByRole("status")).toHaveTextContent("REPORTS");
    expect(props.onChosen).not.toHaveBeenCalled();
  });

  it("says why when the create failed, and stays open", async () => {
    const props = renderDialog({ create: vi.fn(async () => { throw new Error("403 Forbidden"); }) });
    await userEvent.click(screen.getByRole("button", { name: "Create page" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("403 Forbidden");
    expect(props.onChosen).not.toHaveBeenCalled();
    expect(props.onClose).not.toHaveBeenCalled();
  });
});
