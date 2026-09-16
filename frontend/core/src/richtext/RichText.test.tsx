import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RichText, RICH_TEXT_LIMIT } from "./RichText";

describe("RichText blocks", () => {
  it("renders a wiki heading at its semantic level", () => {
    render(<RichText text="h3. Section" format="wiki" onOpenLink={vi.fn()} />);
    expect(screen.getByRole("heading", { level: 3, name: "Section" })).toBeInTheDocument();
  });

  it("renders bullet and ordered lists", () => {
    const { container } = render(<RichText text={"* a\n* b"} format="wiki" onOpenLink={vi.fn()} />);
    expect(container.querySelector("ul")).not.toBeNull();
    expect(container.querySelectorAll("li")).toHaveLength(2);

    const { container: ordered } = render(<RichText text={"# a\n# b"} format="wiki" onOpenLink={vi.fn()} />);
    expect(ordered.querySelector("ol")).not.toBeNull();
  });

  it("renders a table inside its own scroll box", () => {
    const { container } = render(<RichText text={"||H1||H2|\n|a|b|"} format="wiki" onOpenLink={vi.fn()} />);
    const scroll = container.querySelector(".rich-table-scroll");
    expect(scroll).not.toBeNull();
    expect(scroll!.querySelector("table")).not.toBeNull();
    expect(scroll!.querySelector("th")?.textContent).toBe("H1");
    expect(scroll!.querySelector("td")?.textContent).toBe("a");
  });

  it("renders a code block as pre > code", () => {
    const { container } = render(<RichText text={"{code}\nconst x = 1;\n{code}"} format="wiki" onOpenLink={vi.fn()} />);
    const pre = container.querySelector("pre");
    expect(pre).not.toBeNull();
    expect(pre!.querySelector("code")?.textContent).toBe("const x = 1;");
  });

  it("renders a quote as a blockquote and a rule as an hr", () => {
    const { container } = render(<RichText text="bq. Quoted line" format="wiki" onOpenLink={vi.fn()} />);
    expect(container.querySelector("blockquote")?.textContent).toBe("Quoted line");

    const { container: rule } = render(<RichText text="----" format="wiki" onOpenLink={vi.fn()} />);
    expect(rule.querySelector("hr")).not.toBeNull();
  });

  it("renders a panel with its title", () => {
    const { container } = render(
      <RichText text={"{panel:title=Note}\nBody text\n{panel}"} format="wiki" onOpenLink={vi.fn()} />
    );
    const panel = container.querySelector(".rich-panel");
    expect(panel).not.toBeNull();
    expect(panel!.textContent).toContain("Note");
    expect(panel!.textContent).toContain("Body text");
  });

  it("renders a checked task item as a disabled checked checkbox", () => {
    const { container } = render(
      <RichText text={"- [x] done\n- [ ] not done"} format="markdown" onOpenLink={vi.fn()} />
    );
    const boxes = container.querySelectorAll('input[type="checkbox"]');
    expect(boxes).toHaveLength(2);
    expect(boxes[0]).toBeChecked();
    expect(boxes[0]).toBeDisabled();
    expect(boxes[1]).not.toBeChecked();
    expect(boxes[1]).toBeDisabled();
  });

  it("roots the output in a div normally and a span when inline", () => {
    const { container: block } = render(<RichText text="hi" onOpenLink={vi.fn()} />);
    expect(block.firstElementChild?.tagName).toBe("DIV");
    expect(block.firstElementChild).toHaveClass("rich-text");

    const { container: inline } = render(<RichText text="hi" inline onOpenLink={vi.fn()} />);
    expect(inline.firstElementChild?.tagName).toBe("SPAN");
  });
});

describe("RichText link safety", () => {
  const badLinks: Array<[string, "wiki" | "markdown"]> = [
    ["[x|javascript:alert(1)]", "wiki"],
    ["[x|java&#9;script:alert(1)]", "wiki"],
    ["[x|java\tscript:alert(1)]", "wiki"],
    ["[x](javascript:alert(1))", "markdown"],
    ["[x](data:text/html,hi)", "markdown"],
    ["[x](/relative)", "markdown"],
  ];

  it.each(badLinks)("renders %s as text with no button, href or img", (text, format) => {
    const { container } = render(<RichText text={text} format={format} onOpenLink={vi.fn()} />);
    expect(container.textContent).toContain("x");
    expect(container.querySelector("button")).toBeNull();
    expect(container.querySelector("[href]")).toBeNull();
    expect(container.querySelector("img")).toBeNull();
  });

  it("shows a raw <script> tag from Markdown as text, not as markup", () => {
    const { container } = render(<RichText text="<script>alert(1)</script>" format="markdown" onOpenLink={vi.fn()} />);
    expect(container.textContent).toContain("<script>alert(1)</script>");
    expect(container.querySelector("script")).toBeNull();
  });

  it("calls onOpenLink once when an allowed link is clicked, and never navigates", async () => {
    const onOpenLink = vi.fn();
    const originalHref = window.location.href;
    render(<RichText text="[Figma|https://figma.com/f]" format="wiki" onOpenLink={onOpenLink} />);
    const button = screen.getByRole("link", { name: "Figma" });
    await userEvent.click(button);
    expect(onOpenLink).toHaveBeenCalledTimes(1);
    expect(onOpenLink).toHaveBeenCalledWith("https://figma.com/f");
    expect(window.location.href).toBe(originalHref);
  });

  it("allows a mailto link", () => {
    render(<RichText text="[Mail|mailto:team@example.com]" format="wiki" onOpenLink={vi.fn()} />);
    expect(screen.getByRole("link", { name: "Mail" })).toBeInTheDocument();
  });
});

describe("RichText issue keys", () => {
  it("turns the active project's key into a button that calls onIssueKey", async () => {
    const onIssueKey = vi.fn();
    render(<RichText text="see PLAT-409." format="wiki" projectKey="PLAT" onOpenLink={vi.fn()} onIssueKey={onIssueKey} />);
    const button = screen.getByRole("button", { name: "PLAT-409" });
    await userEvent.click(button);
    expect(onIssueKey).toHaveBeenCalledWith("PLAT-409");
  });

  it("leaves keys as text without onIssueKey, and other projects' keys as text always", () => {
    const { container, rerender } = render(
      <RichText text="see XT-9 and PLAT-409." format="wiki" projectKey="PLAT" onOpenLink={vi.fn()} />
    );
    expect(container.textContent).toContain("XT-9");
    expect(container.textContent).toContain("PLAT-409");
    expect(container.querySelector("button")).toBeNull();

    rerender(
      <RichText text="see XT-9 and PLAT-409." format="wiki" projectKey="PLAT" onOpenLink={vi.fn()} onIssueKey={vi.fn()} />
    );
    expect(screen.getByRole("button", { name: "PLAT-409" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "XT-9" })).toBeNull();
  });
});

describe("RichText inline mode", () => {
  it("renders inline code without a wrapping paragraph", () => {
    const { container } = render(<RichText text="apply at {{/payment}} step" format="wiki" inline onOpenLink={vi.fn()} />);
    expect(container.querySelector("p")).toBeNull();
    expect(container.querySelector("code")?.textContent).toBe("/payment");
  });

  it("drops the heading element but keeps its text", () => {
    const { container } = render(<RichText text="h2. Title" format="wiki" inline onOpenLink={vi.fn()} />);
    expect(container.querySelector("h1,h2,h3,h4,h5,h6")).toBeNull();
    expect(container.textContent).toBe("Title");
  });

  it("renders marks as plain text, never bold or italic", () => {
    const { container } = render(<RichText text="*bold* and _em_" format="wiki" inline onOpenLink={vi.fn()} />);
    expect(container.querySelector("strong")).toBeNull();
    expect(container.querySelector("em")).toBeNull();
    expect(container.textContent).toBe("bold and em");
  });

  it("still renders a link as a button", async () => {
    const onOpenLink = vi.fn();
    render(<RichText text="[Figma|https://figma.com/f]" format="wiki" inline onOpenLink={onOpenLink} />);
    await userEvent.click(screen.getByRole("link", { name: "Figma" }));
    expect(onOpenLink).toHaveBeenCalledWith("https://figma.com/f");
  });
});

describe("RichText size guard", () => {
  it("shows the size sentence and caps rendered text at RICH_TEXT_LIMIT", () => {
    const raw = "a".repeat(250_000);
    const { container } = render(<RichText text={raw} format="wiki" onOpenLink={vi.fn()} />);
    const sentence = "Showing the first 200 KB. Open in Jira for the rest.";
    expect(container.textContent).toContain(sentence);
    const rest = (container.textContent ?? "").replace(sentence, "");
    expect(rest.length).toBeLessThanOrEqual(RICH_TEXT_LIMIT);
  });

  it("says nothing about size for text under the limit", () => {
    const { container } = render(<RichText text="short" format="wiki" onOpenLink={vi.fn()} />);
    expect(container.textContent).not.toContain("Showing the first 200 KB");
  });
});
