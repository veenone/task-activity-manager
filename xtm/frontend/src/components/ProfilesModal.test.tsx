import type { ReactNode } from "react";
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import * as api from "../api";
import type { Profile } from "../api";
import { ProfilesModal } from "./ProfilesModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, ListPendingChanges: vi.fn() };
});

// XTM's form loads connections and settings on mount; the delete path needs
// only the footer buttons the dialog hands it.
vi.mock("./ProfileForm", () => ({
  ProfileForm: ({ extraActions }: { extraActions?: ReactNode }) => <div>{extraActions}</div>,
}));

const acme = { id: "p1", name: "Acme Platform", projectKey: "PLAT" } as Profile;

describe("ProfilesModal", () => {
  it("counts the profile's uncommitted changes in the delete confirmation", async () => {
    vi.mocked(api.ListPendingChanges).mockResolvedValue(
      [{ id: 1 }, { id: 2 }, { id: 3 }] as Awaited<ReturnType<typeof api.ListPendingChanges>>,
    );
    render(
      <DialogProvider>
        <ProfilesModal
          profiles={[acme]}
          activeId="p1"
          defaultProfileId=""
          onClose={vi.fn()}
          onSetDefault={vi.fn()}
          onExport={vi.fn()}
          onImport={vi.fn(async () => null)}
          onSaved={vi.fn()}
          onDelete={vi.fn(async () => {})}
        />
      </DialogProvider>,
    );
    await userEvent.click(screen.getByText("Acme Platform"));
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog", { name: /Delete profile/ });
    expect(api.ListPendingChanges).toHaveBeenCalledWith("p1");
    expect(dialog).toHaveTextContent("Its 3 uncommitted changes will be lost.");
  });
});
