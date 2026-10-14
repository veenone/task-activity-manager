import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProfileForm } from "./ProfileForm";
import type { Profile } from "../api";
import { AddConnection, CreateProfile, CreateProfileReusingToken, SetProfileCrossProjectSources, TestConnection } from "../api";

// ProfileForm's profile and connection paths. The Kiwi bug tracker section has
// its own tests in ProfileForm.test.tsx.

vi.mock("../api", () => ({
  CreateProfile: vi.fn(),
  CreateProfileReusingToken: vi.fn(),
  UpdateProfile: vi.fn(),
  SetProfileCrossProjectSources: vi.fn(async () => {}),
  AddConnection: vi.fn(),
  UpdateConnection: vi.fn(),
  TestConnection: vi.fn(),
  TestProfileConnection: vi.fn(),
  GetBugConnection: vi.fn(async () => ({ id: "", url: "", projectKey: "", bugIssueType: "", caCert: "", allowUntrustedTls: false })),
  SaveBugConnection: vi.fn(),
  DeleteBugConnection: vi.fn(),
  errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

vi.mock("./useConfirm", () => ({
  useConfirm: () => ({ confirm: vi.fn(async () => true) }),
}));

const kiwiProfile: Profile = {
  id: "p1",
  name: "Kiwi profile",
  jiraUrl: "https://kiwi.example.com",
  projectKey: "MyProduct",
  scopeJql: "",
  crossProjectSources: "",
  bugIssueType: "",
  bugProjectMode: "test",
  bugProjectKey: "",
  caCert: "",
  allowUntrustedTls: false,
  backend: "kiwi",
  createdAt: "",
};

const xrayProfile: Profile = {
  ...kiwiProfile,
  id: "p2",
  name: "QA main",
  jiraUrl: "https://jira.example.com",
  projectKey: "QA",
  scopeJql: "labels = smoke",
  caCert: "PEM",
  backend: "xray",
};
const mockFn = (fn: unknown) => fn as ReturnType<typeof vi.fn>;

// The main profile and connection paths, pinned before the form was split
// into a folder (#246).
describe("ProfileForm profile and connection", () => {
  beforeEach(() => {
    mockFn(CreateProfile).mockReset().mockImplementation(async () => ({ ...xrayProfile, id: "new" }));
    mockFn(TestConnection).mockReset().mockResolvedValue("ann");
    mockFn(AddConnection).mockReset().mockResolvedValue({ id: "c1" });
    mockFn(SetProfileCrossProjectSources).mockClear();
  });

  it("creates an Xray profile from the typed fields", async () => {
    const onCreated = vi.fn();
    render(<ProfileForm onCreated={onCreated} />);
    await userEvent.type(screen.getByLabelText("Profile name"), "QA");
    await userEvent.type(screen.getByLabelText("Jira base URL"), "https://jira.example.com/");
    await userEvent.type(screen.getByLabelText("Project key"), "qa");
    await userEvent.selectOptions(screen.getByLabelText("Bug project"), "dedicated");
    await userEvent.type(screen.getByLabelText("Dedicated bug project key"), "def");
    await userEvent.type(screen.getByLabelText(/Cross-project source projects/), "VAL");
    await userEvent.type(screen.getByLabelText("Personal Access Token"), "pat");
    await userEvent.click(screen.getByRole("button", { name: "Create profile" }));
    expect(CreateProfile).toHaveBeenCalledWith(
      "QA", "https://jira.example.com", "QA", "", "", "dedicated", "DEF", "pat", "", false, "xray",
    );
    expect(SetProfileCrossProjectSources).toHaveBeenCalledWith("new", "VAL");
    expect(onCreated).toHaveBeenCalledWith(expect.objectContaining({ id: "new" }));
  });

  it("blocks save on a malformed URL or project key", async () => {
    render(<ProfileForm onCreated={() => {}} />);
    await userEvent.type(screen.getByLabelText("Profile name"), "QA");
    await userEvent.type(screen.getByLabelText("Jira base URL"), "jira example");
    await userEvent.type(screen.getByLabelText("Project key"), "Q A");
    await userEvent.type(screen.getByLabelText("Personal Access Token"), "pat");
    expect(screen.getByText("The URL must not contain spaces.")).toBeInTheDocument();
    expect(screen.getByText(/Project key must start with a letter/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create profile" })).toBeDisabled();
  });

  it("tests a new connection with the typed token", async () => {
    render(<ProfileForm onCreated={() => {}} />);
    await userEvent.type(screen.getByLabelText("Jira base URL"), "https://jira.example.com");
    await userEvent.type(screen.getByLabelText("Personal Access Token"), "pat");
    await userEvent.click(screen.getByRole("button", { name: "Test connection" }));
    expect(TestConnection).toHaveBeenCalledWith("https://jira.example.com", "pat", "", false, "xray");
    expect(await screen.findByText("Connected as ann")).toBeInTheDocument();
  });

  it("starts from a copy of another profile and reuses its token", async () => {
    mockFn(CreateProfileReusingToken).mockResolvedValue({ ...xrayProfile, id: "copy" });
    render(<ProfileForm onCreated={() => {}} profiles={[xrayProfile, kiwiProfile]} />);
    expect(screen.queryByRole("option", { name: /Kiwi profile/ })).toBeNull();
    await userEvent.selectOptions(screen.getByLabelText(/Start from/), "p2");
    expect(screen.getByLabelText("Jira base URL")).toHaveValue("https://jira.example.com");
    expect(screen.queryByPlaceholderText(/Jira PAT/)).toBeNull();
    await userEvent.type(screen.getByLabelText("Profile name"), "QA copy");
    await userEvent.click(screen.getByRole("button", { name: "Create profile" }));
    expect(CreateProfileReusingToken).toHaveBeenCalledWith(
      "QA copy", "https://jira.example.com", "QA", "labels = smoke", "", "test", "", "p2", "xray", "PEM", false,
    );
  });

  it("asks for both halves of a Kiwi credential", async () => {
    render(<ProfileForm onCreated={() => {}} />);
    await userEvent.selectOptions(screen.getByLabelText(/backend/i), "kiwi");
    await userEvent.type(screen.getByLabelText("Kiwi username"), "ann");
    expect(screen.getByText(/Enter both username and password/)).toBeInTheDocument();
  });

  it("adds a connection with its role in connection mode", async () => {
    const onSaved = vi.fn();
    render(<ProfileForm mode="connection" workspaceId="w1" onSaved={onSaved} />);
    await userEvent.type(screen.getByLabelText("Connection name"), "Kiwi target");
    await userEvent.selectOptions(screen.getByLabelText("Role"), "both");
    await userEvent.type(screen.getByLabelText("Jira base URL"), "https://jira.example.com");
    await userEvent.type(screen.getByLabelText("Project key"), "QA");
    await userEvent.type(screen.getByLabelText("Personal Access Token"), "pat");
    await userEvent.click(screen.getByRole("button", { name: "Add connection" }));
    expect(AddConnection).toHaveBeenCalledWith(
      "w1", "Kiwi target", "xray", "https://jira.example.com", "QA", "", "", "test", "", "pat", "", false, "both",
    );
    expect(onSaved).toHaveBeenCalledWith({ id: "c1" });
  });

  it("warns that changing the project key clears the cache", async () => {
    render(<ProfileForm onCreated={() => {}} profile={xrayProfile} />);
    expect(screen.queryByText(/clears this profile's cached/)).toBeNull();
    await userEvent.clear(screen.getByLabelText("Project key"));
    await userEvent.type(screen.getByLabelText("Project key"), "QB");
    expect(screen.getByText(/clears this profile's cached/)).toBeInTheDocument();
  });
});
