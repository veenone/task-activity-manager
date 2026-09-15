import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as api from "../api";
import type { Profile } from "../api";
import { ROOT_FIX_BEFORE_SAVE, ROOT_ID_NOT_A_NUMBER } from "../lib/confluenceRoot";
import { ProfileForm } from "./ProfileForm";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    UpdateProfile: vi.fn(),
    TestConnection: vi.fn(),
    TestProfileConnection: vi.fn(),
    GetProfileSetting: vi.fn(),
    SetProfileSetting: vi.fn(),
    GetConfluenceConfig: vi.fn(),
    SetConfluenceConfig: vi.fn(),
  };
});

const acme: Profile = {
  id: "p1",
  name: "Acme Platform",
  jiraUrl: "https://jira.acme.example",
  projectKey: "PLAT",
  backend: "xray",
  createdAt: "",
  scopeJql: "",
  caCert: "",
  allowUntrustedTls: false,
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.GetProfileSetting).mockResolvedValue("");
  vi.mocked(api.SetProfileSetting).mockResolvedValue();
  vi.mocked(api.UpdateProfile).mockResolvedValue(acme);
  vi.mocked(api.GetConfluenceConfig).mockResolvedValue({ baseURL: "https://confluence.example.com", spaceKey: "TEAM", rootPageID: "653264152" });
  vi.mocked(api.SetConfluenceConfig).mockResolvedValue();
});

describe("ProfileForm's Confluence root page id", () => {
  it("reads a pasted page address as its page id and saves the id", async () => {
    const onSaved = vi.fn();
    render(<ProfileForm profile={acme} onSaved={onSaved} />);
    const root = await screen.findByDisplayValue("653264152");
    await userEvent.clear(root);
    await userEvent.type(root, "https://confluence.example.com/pages/viewpage.action?pageId=42");
    await userEvent.tab();
    expect(root).toHaveValue("42");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() =>
      expect(api.SetConfluenceConfig).toHaveBeenCalledWith("p1", { baseURL: "https://confluence.example.com", spaceKey: "TEAM", rootPageID: "42" }, ""),
    );
    expect(onSaved).toHaveBeenCalled();
  });

  it("refuses a root page id that is not a number, and says so beside Save", async () => {
    render(<ProfileForm profile={acme} onSaved={vi.fn()} />);
    const root = await screen.findByDisplayValue("653264152");
    await userEvent.clear(root);
    await userEvent.type(root, "Team rituals");
    expect(screen.getByText(ROOT_ID_NOT_A_NUMBER)).toBeInTheDocument();
    expect(screen.getByText(ROOT_FIX_BEFORE_SAVE)).toBeInTheDocument();
    expect(root).toHaveAttribute("aria-invalid", "true");
    expect(root).toHaveAccessibleDescription(ROOT_ID_NOT_A_NUMBER);
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
  });
});

describe("ProfileForm's field errors", () => {
  it("ties the Jira URL and project key errors to their inputs", async () => {
    render(<ProfileForm profile={acme} onSaved={vi.fn()} />);
    const url = await screen.findByDisplayValue("https://jira.acme.example");
    await userEvent.clear(url);
    await userEvent.type(url, "jira acme");
    expect(url).toHaveAttribute("aria-invalid", "true");
    expect(url).toHaveAccessibleDescription("The URL must not contain spaces.");

    const key = screen.getByDisplayValue("PLAT");
    expect(key).not.toHaveAttribute("aria-describedby");
    await userEvent.type(key, "/X");
    expect(key).toHaveAttribute("aria-invalid", "true");
    expect(key).toHaveAccessibleDescription(/^Project key must start with a letter/);
  });
});
