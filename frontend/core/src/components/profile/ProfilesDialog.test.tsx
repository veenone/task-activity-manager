import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "../../contexts/DialogContext";
import { ProfilesDialog } from "./ProfilesDialog";
import type { ProfileFormSlot } from "./ProfilesDialog";

interface P {
  id: string;
  name: string;
  projectKey: string;
}
const acme: P = { id: "p1", name: "Acme", projectKey: "PLAT" };
const beta: P = { id: "p2", name: "Beta", projectKey: "BETA" };

// A stand-in for an app's ProfileForm: it names what it was given and saves a
// fixed profile, which is all the dialog needs from it.
function form({ profile, onSaved, onCancel, extraActions }: ProfileFormSlot<P>) {
  return (
    <div>
      <p>{profile ? `form for ${profile.name}` : "form for a new profile"}</p>
      <button onClick={() => onSaved(profile ?? { id: "p9", name: "Fresh", projectKey: "NEW" })}>Save</button>
      {onCancel && <button onClick={onCancel}>Cancel form</button>}
      {extraActions}
    </div>
  );
}

function setup(over: Partial<Parameters<typeof ProfilesDialog<P>>[0]> = {}) {
  const props = {
    profiles: [acme, beta],
    activeId: "p1",
    defaultProfileId: "p2",
    deleteMessage: "This removes its token.",
    onSetDefault: vi.fn(async () => {}),
    onExport: vi.fn(async () => {}),
    onImport: vi.fn(async () => beta as P | null),
    onSaved: vi.fn(async () => {}),
    onDelete: vi.fn(async () => {}),
    onClose: vi.fn(),
    renderForm: form,
    ...over,
  };
  render(
    <DialogProvider>
      <ProfilesDialog<P> {...props} />
    </DialogProvider>,
  );
  return props;
}

describe("ProfilesDialog", () => {
  it("opens on the start state and opens a profile picked from the list", async () => {
    const user = userEvent.setup();
    setup();
    expect(screen.getByRole("heading", { name: "Nothing open yet" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Beta/ }));
    expect(screen.getByText("form for Beta")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Beta/ })).toHaveAttribute("aria-current", "true");
  });

  it("warns before editing the active profile", async () => {
    const user = userEvent.setup();
    setup();
    await user.click(screen.getByRole("button", { name: /Acme/ }));
    expect(screen.getByText("This is your active profile.")).toBeInTheDocument();
  });

  it("creates a profile through the form and opens it", async () => {
    const user = userEvent.setup();
    const props = setup();
    await user.click(screen.getAllByRole("button", { name: "Create new profile" })[0]);
    expect(screen.getByText("form for a new profile")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(props.onSaved).toHaveBeenCalledWith({ id: "p9", name: "Fresh", projectKey: "NEW" });
  });

  it("opens an imported profile, and stays put when the import is cancelled", async () => {
    const user = userEvent.setup();
    const onImport = vi.fn(async () => beta as P | null);
    setup({ onImport });
    await user.click(screen.getByRole("button", { name: "Import from file…" }));
    expect(await screen.findByText("form for Beta")).toBeInTheDocument();
    onImport.mockResolvedValueOnce(null);
    await user.click(screen.getByRole("button", { name: "Import from file…" }));
    expect(screen.getByText("form for Beta")).toBeInTheDocument();
  });

  it("toggles the launch default with the star", async () => {
    const user = userEvent.setup();
    const props = setup();
    // The default is the app's: Beta's star reads as set, Acme's as not.
    expect(screen.getAllByRole("button", { name: "Clear default" })).toHaveLength(1);
    await user.click(screen.getByRole("button", { name: "Set as default" }));
    expect(props.onSetDefault).toHaveBeenCalledWith("p1");
  });

  it("exports the open profile", async () => {
    const user = userEvent.setup();
    const props = setup();
    await user.click(screen.getByRole("button", { name: /Beta/ }));
    await user.click(screen.getByRole("button", { name: "Export" }));
    expect(props.onExport).toHaveBeenCalledWith("p2");
  });

  it("deletes only after the confirmation, which carries the app's warning", async () => {
    const user = userEvent.setup();
    const props = setup();
    await user.click(screen.getByRole("button", { name: /Beta/ }));
    await user.click(screen.getByRole("button", { name: "Delete" }));
    const confirm = await screen.findByRole("alertdialog");
    expect(within(confirm).getByText("This removes its token.")).toBeInTheDocument();
    await user.click(within(confirm).getByRole("button", { name: "Delete profile" }));
    expect(props.onDelete).toHaveBeenCalledWith("p2");
  });

  // Deleting a profile also purges its pending-change journal, so the
  // confirmation counts what is about to be lost (#523).
  async function confirmText(pendingCount: (id: string) => Promise<number>) {
    const user = userEvent.setup();
    const props = setup({ pendingCount });
    await user.click(screen.getByRole("button", { name: /Beta/ }));
    await user.click(screen.getByRole("button", { name: "Delete" }));
    const confirm = await screen.findByRole("alertdialog");
    expect(pendingCount).toHaveBeenCalledWith("p2");
    return { text: confirm.textContent, props };
  }

  it("says how many uncommitted changes the delete discards", async () => {
    const { text } = await confirmText(vi.fn(async () => 3));
    expect(text).toContain("This removes its token.");
    expect(text).toContain("Its 3 uncommitted changes will be lost. This removes its token.");
  });

  it("uses the singular for one uncommitted change", async () => {
    const { text } = await confirmText(vi.fn(async () => 1));
    expect(text).toContain("Its 1 uncommitted change will be lost.");
  });

  it("adds nothing when the profile has no uncommitted changes", async () => {
    const { text } = await confirmText(vi.fn(async () => 0));
    expect(text).not.toContain("uncommitted");
  });

  it("still warns, and still deletes, when the changes cannot be counted", async () => {
    const user = userEvent.setup();
    const { text, props } = await confirmText(vi.fn(async () => Promise.reject(new Error("locked"))));
    expect(text).toContain("Any uncommitted changes it has will be lost.");
    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "Delete profile" }));
    expect(props.onDelete).toHaveBeenCalledWith("p2");
  });

  it("shows a failed callback's error in the dialog", async () => {
    const user = userEvent.setup();
    setup({ onExport: vi.fn(async () => Promise.reject(new Error("disk full"))) });
    await user.click(screen.getByRole("button", { name: /Beta/ }));
    await user.click(screen.getByRole("button", { name: "Export" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("disk full");
  });

  // Escape or a click outside used to close the dialog over a typed URL and
  // token without a word (#542).
  it("asks before Escape drops a form holding unsaved input", async () => {
    const user = userEvent.setup();
    const typingForm = ({ onDirtyChange }: ProfileFormSlot<P>) => (
      <input aria-label="Jira base URL" onChange={(e) => onDirtyChange?.(e.target.value !== "")} />
    );
    const props = setup({ renderForm: typingForm });
    await user.click(screen.getByRole("button", { name: /Beta/ }));
    await user.type(screen.getByLabelText("Jira base URL"), "https://jira.example.com");
    await user.keyboard("{Escape}");

    const ask = await screen.findByRole("alertdialog");
    expect(ask).toHaveTextContent("Discard your changes to this profile?");
    await user.click(within(ask).getByRole("button", { name: "Keep editing" }));
    expect(props.onClose).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Jira base URL")).toHaveValue("https://jira.example.com");

    await user.click(screen.getByRole("button", { name: "Close" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Discard" }));
    expect(props.onClose).toHaveBeenCalledTimes(1);
  });

  it("closes on Escape without asking when nothing was typed", async () => {
    const user = userEvent.setup();
    const props = setup();
    await user.click(screen.getByRole("button", { name: /Beta/ }));
    await user.keyboard("{Escape}");
    expect(props.onClose).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("alertdialog")).toBeNull();
  });

  it("carries the app's subtitle sentence and list note", () => {
    setup({ subtitle: "The list is shared.", listNote: "Kiwi profiles are not listed." });
    expect(screen.getByText(/The list is shared\./)).toBeInTheDocument();
    expect(screen.getByText("Kiwi profiles are not listed.")).toBeInTheDocument();
  });
});
