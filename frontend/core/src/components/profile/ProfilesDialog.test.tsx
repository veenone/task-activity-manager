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

  it("shows a failed callback's error in the dialog", async () => {
    const user = userEvent.setup();
    setup({ onExport: vi.fn(async () => Promise.reject(new Error("disk full"))) });
    await user.click(screen.getByRole("button", { name: /Beta/ }));
    await user.click(screen.getByRole("button", { name: "Export" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("disk full");
  });

  it("carries the app's subtitle sentence and list note", () => {
    setup({ subtitle: "The list is shared.", listNote: "Kiwi profiles are not listed." });
    expect(screen.getByText(/The list is shared\./)).toBeInTheDocument();
    expect(screen.getByText("Kiwi profiles are not listed.")).toBeInTheDocument();
  });
});
