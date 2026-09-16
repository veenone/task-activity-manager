import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RichTextField, RICH_TEXT_SENTENCES, SyntaxToggle } from "./RichTextField";
import type { RichFormat } from "./ast";

// A minimal controlled harness: RichTextField takes value/format and their
// setters as props, same as EditableFields (Task 7) will hold them, so tests
// exercise the real controlled loop rather than an internal-state stand-in.
function ControlledField(props: {
  spy?: (f: RichFormat) => void;
  initialValue?: string;
  initialFormat?: RichFormat | "auto";
  textarea?: Record<string, unknown>;
}) {
  const [value, setValue] = useState(props.initialValue ?? "");
  const [format, setFormat] = useState<RichFormat | "auto">(props.initialFormat ?? "auto");
  return (
    <RichTextField
      value={value}
      onChange={setValue}
      format={format}
      onFormatChange={(f) => {
        props.spy?.(f);
        setFormat(f);
      }}
      onOpenLink={vi.fn()}
      textarea={props.textarea}
    />
  );
}

describe("RichTextField tabs", () => {
  it("is a tablist with Write and Preview tabs and a single tabpanel", () => {
    render(<ControlledField />);
    expect(screen.getByRole("tablist")).toBeInTheDocument();
    const write = screen.getByRole("tab", { name: "Write" });
    const preview = screen.getByRole("tab", { name: "Preview" });
    expect(write).toHaveAttribute("aria-selected", "true");
    expect(preview).toHaveAttribute("aria-selected", "false");
    expect(screen.getByRole("tabpanel")).toBeInTheDocument();
  });

  it("Ctrl+Shift+P from the textarea switches to Preview and back", () => {
    render(<ControlledField />);
    const textarea = screen.getByRole("textbox");
    fireEvent.keyDown(textarea, { key: "P", ctrlKey: true, shiftKey: true });
    expect(screen.getByRole("tab", { name: "Preview" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "Write" })).toHaveAttribute("aria-selected", "false");

    fireEvent.keyDown(textarea, { key: "P", ctrlKey: true, shiftKey: true });
    expect(screen.getByRole("tab", { name: "Write" })).toHaveAttribute("aria-selected", "true");
  });

  it("ArrowRight and ArrowLeft move focus and selection between the tabs", async () => {
    const user = userEvent.setup();
    render(<ControlledField />);
    const write = screen.getByRole("tab", { name: "Write" });
    const preview = screen.getByRole("tab", { name: "Preview" });
    write.focus();
    await user.keyboard("{ArrowRight}");
    expect(preview).toHaveFocus();
    expect(preview).toHaveAttribute("aria-selected", "true");
    expect(preview).toHaveAttribute("tabindex", "0");
    expect(write).toHaveAttribute("tabindex", "-1");
    await user.keyboard("{ArrowLeft}");
    expect(write).toHaveFocus();
    expect(write).toHaveAttribute("aria-selected", "true");
  });

  it("keeps the textarea reachable from an outside label while Preview is shown, and switches back to Write on focus", async () => {
    const user = userEvent.setup();
    function Wrapper() {
      const [value, setValue] = useState("");
      const [format, setFormat] = useState<RichFormat | "auto">("auto");
      return (
        <>
          <label htmlFor="desc-field">Description</label>
          <RichTextField
            value={value}
            onChange={setValue}
            format={format}
            onFormatChange={setFormat}
            onOpenLink={vi.fn()}
            textarea={{ id: "desc-field" }}
          />
        </>
      );
    }
    render(<Wrapper />);
    await user.click(screen.getByRole("tab", { name: "Preview" }));
    expect(screen.getByRole("tab", { name: "Preview" })).toHaveAttribute("aria-selected", "true");
    const textarea = screen.getByLabelText("Description");
    expect(textarea).not.toHaveAttribute("hidden");

    await user.click(screen.getByText("Description"));
    expect(textarea).toHaveFocus();
    expect(screen.getByRole("tab", { name: "Write" })).toHaveAttribute("aria-selected", "true");
  });
});

describe("RichTextField auto detection and the syntax toggle", () => {
  it("shows the detected-wiki hint naming h3. in auto mode", async () => {
    const user = userEvent.setup();
    render(<ControlledField />);
    const textarea = screen.getByRole("textbox");
    await user.click(textarea);
    await user.type(textarea, "h3. Title");
    expect(screen.getByText(/Detected Jira markup \(h3\.\)/)).toBeInTheDocument();
  });

  it("flips to Markdown on toggle and shows the Markdown warning", async () => {
    const user = userEvent.setup();
    render(<ControlledField initialValue="## Title" />);
    await user.click(screen.getByRole("button", { name: "Markdown" }));
    expect(screen.getByText(RICH_TEXT_SENTENCES.markdownWarning)).toBeInTheDocument();
    expect(screen.getByText(RICH_TEXT_SENTENCES.picked("markdown"))).toBeInTheDocument();
  });

  it("reads as Jira markup with no signal", () => {
    render(<ControlledField />);
    expect(screen.getByText(RICH_TEXT_SENTENCES.none)).toBeInTheDocument();
  });

  it("picking Markdown calls onFormatChange, and the parent's picked value survives new wiki signals", async () => {
    const spy = vi.fn();
    const user = userEvent.setup();
    render(<ControlledField spy={spy} initialValue="plain text" />);
    await user.click(screen.getByRole("button", { name: "Markdown" }));
    expect(spy).toHaveBeenCalledWith("markdown");

    const textarea = screen.getByRole("textbox");
    await user.click(textarea);
    await user.type(textarea, "{End}\nh3. Wiki heading");
    expect(screen.getByText(RICH_TEXT_SENTENCES.picked("markdown"))).toBeInTheDocument();
  });
});

describe("RichTextField typing and rows", () => {
  it("passes the exact typed string to onChange, including a trailing newline", async () => {
    const user = userEvent.setup();
    const handled: string[] = [];
    function Wrapper() {
      const [value, setValue] = useState("");
      return (
        <RichTextField
          value={value}
          onChange={(v) => {
            handled.push(v);
            setValue(v);
          }}
          format="wiki"
          onFormatChange={vi.fn()}
          onOpenLink={vi.fn()}
        />
      );
    }
    render(<Wrapper />);
    const textarea = screen.getByRole("textbox");
    await user.click(textarea);
    await user.type(textarea, "*bold*\n");
    expect(handled[handled.length - 1]).toBe("*bold*\n");
  });

  it("grows rows from 4 up to a cap of 16 as lines are added", () => {
    render(<ControlledField />);
    const textarea = screen.getByRole("textbox");
    expect(textarea).toHaveAttribute("rows", "4");

    const eightLines = Array.from({ length: 8 }, (_, i) => `line ${i}`).join("\n");
    fireEvent.change(textarea, { target: { value: eightLines } });
    expect(textarea).toHaveAttribute("rows", "8");

    const thirtyLines = Array.from({ length: 30 }, (_, i) => `line ${i}`).join("\n");
    fireEvent.change(textarea, { target: { value: thirtyLines } });
    expect(textarea).toHaveAttribute("rows", "16");
  });
});

describe("RichTextField preview and textarea props", () => {
  it("renders Preview with the chosen format, and lands id/aria-invalid on the textarea", async () => {
    const user = userEvent.setup();
    render(<ControlledField initialValue="h3. Title" initialFormat="wiki" textarea={{ id: "desc", "aria-invalid": true }} />);
    const textarea = screen.getByRole("textbox");
    expect(textarea).toHaveAttribute("id", "desc");
    expect(textarea).toHaveAttribute("aria-invalid", "true");

    await user.click(screen.getByRole("tab", { name: "Preview" }));
    expect(screen.getByRole("heading", { level: 3, name: "Title" })).toBeInTheDocument();
  });
});

describe("SyntaxToggle", () => {
  it("is a group labelled Syntax with two aria-pressed buttons", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<SyntaxToggle value="wiki" onChange={onChange} />);
    const group = screen.getByRole("group", { name: "Syntax" });
    const wiki = within(group).getByRole("button", { name: "Jira markup" });
    const markdown = within(group).getByRole("button", { name: "Markdown" });
    expect(wiki).toHaveAttribute("aria-pressed", "true");
    expect(markdown).toHaveAttribute("aria-pressed", "false");
    await user.click(markdown);
    expect(onChange).toHaveBeenCalledWith("markdown");
  });

  it("disables both buttons when disabled", () => {
    render(<SyntaxToggle value="wiki" onChange={vi.fn()} disabled />);
    for (const button of screen.getAllByRole("button")) expect(button).toBeDisabled();
  });
});

describe("RICH_TEXT_SENTENCES", () => {
  it("carries no em dash in any sentence", () => {
    const values = [
      RICH_TEXT_SENTENCES.detected("wiki", ["h3."]),
      RICH_TEXT_SENTENCES.detected("markdown", ["#"]),
      RICH_TEXT_SENTENCES.none,
      RICH_TEXT_SENTENCES.picked("wiki"),
      RICH_TEXT_SENTENCES.picked("markdown"),
      RICH_TEXT_SENTENCES.markdownWarning,
      RICH_TEXT_SENTENCES.sizeGuard,
    ];
    for (const value of values) expect(value).not.toMatch(/—/);
  });
});
