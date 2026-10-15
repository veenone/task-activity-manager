import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { useProfileForm } from "./useProfileForm";
import type { ProfileFormAdapter, ProfileFormOptions, ReuseCandidate } from "./useProfileForm";
import { KeyField, NameField, ScopeField, UrlField } from "./fields";
import { TokenFields } from "./TokenFields";
import { TlsDetails } from "./TlsDetails";
import { ProfileFormFooter } from "./ProfileFormFooter";

// The core form is composed by each app; this harness composes it the plain
// way, with an adapter that records what the form hands it.
interface Seed { name: string; url: string; key: string; scope: string; caCert: string; allowTls: boolean }

function Harness(props: Partial<ProfileFormOptions<ReuseCandidate>> & { adapter: ProfileFormAdapter; baseline?: Seed }) {
  const f = useProfileForm<ReuseCandidate>({
    isDemoUrl: (u) => u === "demo",
    seed: props.baseline,
    isEdit: !!props.baseline,
    cacheBaseline: props.baseline ? { key: props.baseline.key, url: props.baseline.url } : undefined,
    ...props,
  });
  return (
    <div>
      <NameField f={f} label="Profile name" placeholder="Team: Project X" />
      <UrlField f={f} label="Jira base URL" placeholder="https://jira.example.com" />
      <KeyField f={f} label="Project key" placeholder="QA" />
      <ScopeField f={f} placeholder="e.g. labels = smoke" />
      <TokenFields
        f={f}
        reuse={{ label: "Personal Access Token", blank: "Enter a new token…", option: (p) => `Reuse token from: ${p.name} (${p.projectKey})` }}
      />
      <TlsDetails f={f} summary="Advanced: TLS / certificate settings" hint="Paste a CA." />
      <ProfileFormFooter f={f} createLabel="Create profile" cacheWarning="Changing the key clears the cache." onCancel={() => {}} />
    </div>
  );
}

const adapter = (over: Partial<ProfileFormAdapter> = {}): ProfileFormAdapter => ({
  test: vi.fn(async () => "alice"),
  save: vi.fn(async () => undefined),
  ...over,
});

async function fillValid() {
  await userEvent.type(screen.getByLabelText("Profile name"), "  QA  ");
  await userEvent.type(screen.getByLabelText("Jira base URL"), "https://jira.example.com/");
  await userEvent.type(screen.getByLabelText("Project key"), "qa");
  await userEvent.type(screen.getByLabelText("Personal Access Token"), " pat ");
}

describe("core ProfileForm", () => {
  it("keeps Save off until the required fields are valid, then saves them trimmed and upper-cased", async () => {
    const a = adapter();
    render(<Harness adapter={a} />);
    const save = screen.getByRole("button", { name: "Create profile" });
    expect(save).toBeDisabled();
    await fillValid();
    expect(save).toBeEnabled();
    await userEvent.click(save);
    await waitFor(() => expect(a.save).toHaveBeenCalledTimes(1));
    expect(a.save).toHaveBeenCalledWith({
      name: "QA", url: "https://jira.example.com", key: "QA", scope: "",
      token: "pat", caCert: "", allowTls: false, reuseFrom: "",
    });
  });

  it("links the URL and key errors to their inputs", async () => {
    render(<Harness adapter={adapter()} />);
    const url = screen.getByLabelText("Jira base URL");
    await userEvent.type(url, "jira example");
    expect(url).toHaveAttribute("aria-invalid", "true");
    expect(document.getElementById(url.getAttribute("aria-describedby")!)).toHaveTextContent("must not contain spaces");
    const key = screen.getByLabelText("Project key");
    await userEvent.type(key, "q/a");
    expect(key).toHaveAttribute("aria-invalid", "true");
    expect(document.getElementById(key.getAttribute("aria-describedby")!)).toHaveTextContent("Project key must start");
  });

  it("leaves a free-form key alone in free mode", async () => {
    const a = adapter();
    render(<Harness adapter={a} keyMode="free" />);
    await userEvent.type(screen.getByLabelText("Profile name"), "Kiwi");
    await userEvent.type(screen.getByLabelText("Jira base URL"), "https://kiwi.example.com");
    await userEvent.type(screen.getByLabelText("Project key"), "My Product");
    await userEvent.type(screen.getByLabelText("Personal Access Token"), "u:p");
    expect(screen.queryByText(/Project key must start/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Create profile" }));
    await waitFor(() => expect(a.save).toHaveBeenCalled());
    expect(vi.mocked(a.save).mock.calls[0][0].key).toBe("My Product");
  });

  it("tests a typed token, and the stored one when editing with the token blank", async () => {
    const a = adapter();
    const { unmount } = render(<Harness adapter={a} />);
    await fillValid();
    await userEvent.click(screen.getByRole("button", { name: "Test connection" }));
    expect(await screen.findByText("Connected as alice")).toBeInTheDocument();
    expect(a.test).toHaveBeenLastCalledWith(expect.objectContaining({ token: "pat", stored: false }));
    unmount();

    const b = adapter();
    render(<Harness adapter={b} baseline={{ name: "QA", url: "https://jira.example.com", key: "QA", scope: "", caCert: "", allowTls: false }} />);
    await userEvent.click(screen.getByRole("button", { name: "Test connection" }));
    await screen.findByText("Connected as alice");
    expect(b.test).toHaveBeenLastCalledWith(expect.objectContaining({ token: "", stored: true }));
  });

  it("shows a failed test and a failed save as text", async () => {
    render(<Harness adapter={adapter({ test: vi.fn(async () => { throw new Error("401 bad token"); }), save: vi.fn(async () => { throw new Error("name taken"); }) })} />);
    await fillValid();
    await userEvent.click(screen.getByRole("button", { name: "Test connection" }));
    expect(await screen.findByText("401 bad token")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Create profile" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("name taken");
  });

  it("reports a warning the adapter returns after a save that went through", async () => {
    render(<Harness adapter={adapter({ save: vi.fn(async () => "Saved, but the extra write failed") })} />);
    await fillValid();
    await userEvent.click(screen.getByRole("button", { name: "Create profile" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Saved, but the extra write failed");
  });

  it("accepts a demo URL without a token when the app allows it", async () => {
    render(<Harness adapter={adapter()} demoNeedsNoToken />);
    await userEvent.type(screen.getByLabelText("Profile name"), "Demo");
    await userEvent.type(screen.getByLabelText("Jira base URL"), "demo");
    await userEvent.type(screen.getByLabelText("Project key"), "QA");
    expect(screen.getByLabelText("Personal Access Token")).toHaveAttribute("placeholder", "not needed for a demo profile");
    expect(screen.getByRole("button", { name: "Create profile" })).toBeEnabled();
  });

  it("reuses another profile's token instead of asking for one", async () => {
    const a = adapter();
    const others = [{ id: "p2", name: "Other", projectKey: "OTH" }];
    render(<Harness adapter={a} reuseCandidates={others} />);
    await userEvent.selectOptions(screen.getByLabelText("Personal Access Token"), "p2");
    expect(screen.queryByLabelText("New token")).not.toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Profile name"), "QA");
    await userEvent.type(screen.getByLabelText("Jira base URL"), "https://jira.example.com");
    await userEvent.type(screen.getByLabelText("Project key"), "QA");
    await userEvent.click(screen.getByRole("button", { name: "Create profile" }));
    await waitFor(() => expect(a.save).toHaveBeenCalled());
    expect(vi.mocked(a.save).mock.calls[0][0].reuseFrom).toBe("p2");
  });

  it("warns that an edited key or URL clears the cache", async () => {
    render(<Harness adapter={adapter()} baseline={{ name: "QA", url: "https://jira.example.com", key: "QA", scope: "", caCert: "", allowTls: false }} />);
    expect(screen.queryByText("Changing the key clears the cache.")).not.toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Project key"), "X");
    expect(screen.getByText("Changing the key clears the cache.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeEnabled();
  });

  it("lets an app replace the credential and its checks", async () => {
    const a = adapter();
    render(<Harness adapter={a} credential={{ value: "", entered: false, invalid: true }} />);
    await userEvent.type(screen.getByLabelText("Profile name"), "QA");
    await userEvent.type(screen.getByLabelText("Jira base URL"), "https://jira.example.com");
    await userEvent.type(screen.getByLabelText("Project key"), "QA");
    expect(screen.getByRole("button", { name: "Create profile" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Test connection" })).toBeDisabled();
  });

  it("holds Save while the app's own fields are invalid", async () => {
    render(<Harness adapter={adapter()} extraValid={false} />);
    await fillValid();
    expect(screen.getByRole("button", { name: "Create profile" })).toBeDisabled();
  });
});
