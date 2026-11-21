import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Pager } from "./Pager";

// A pager over 1,248 rows that keeps its own page and size, the way every
// caller does.
function Harness({ onPage = () => {}, ...over }: Partial<Parameters<typeof Pager>[0]>) {
  const [page, setPage] = useState(0);
  const [size, setSize] = useState(25);
  return (
    <Pager
      page={page}
      pageSize={size}
      total={1248}
      onPage={(p) => {
        setPage(p);
        onPage(p);
      }}
      onPageSize={(n) => {
        setSize(n);
        setPage(0);
      }}
      pageSizeOptions={[25, 100]}
      {...over}
    />
  );
}

const range = (a: number, b: number, total: number) =>
  `${a.toLocaleString()} to ${b.toLocaleString()} of ${total.toLocaleString()}`;

describe("Pager", () => {
  it("steps, jumps to either end and says which rows are on screen", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    expect(screen.getByText(range(1, 25, 1248))).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Next page" }));
    expect(screen.getByText(range(26, 50, 1248))).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Last page" }));
    expect(screen.getByText(range(1226, 1248, 1248))).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Next page" })).toBeDisabled();
    expect(screen.getByRole("spinbutton", { name: "Page" })).toHaveValue(50);
    expect(screen.getByText("of 50")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "First page" }));
    expect(screen.getByText(range(1, 25, 1248))).toBeInTheDocument();
  });

  it("goes to a typed page, and leaves a half-typed or out-of-range one alone", async () => {
    const user = userEvent.setup();
    const onPage = vi.fn();
    render(<Harness onPage={onPage} />);
    const box = screen.getByRole("spinbutton", { name: "Page" });
    await user.clear(box);
    expect(box).toHaveValue(null);
    await user.type(box, "23");
    expect(onPage).toHaveBeenLastCalledWith(22);
    expect(screen.getByText(range(551, 575, 1248))).toBeInTheDocument();

    onPage.mockClear();
    await user.clear(box);
    await user.type(box, "99");
    expect(onPage).toHaveBeenLastCalledWith(8);
    expect(onPage).not.toHaveBeenCalledWith(98);
  });

  it("changes the page size through the select it names", async () => {
    const user = userEvent.setup();
    render(<Harness sizeName="Issue groups per page" sizeLabel={(n) => `${n} groups / page`} />);
    const select = screen.getByRole("combobox", { name: "Issue groups per page" });
    expect([...(select as HTMLSelectElement).options].map((o) => o.textContent)).toEqual(["25 groups / page", "100 groups / page"]);
    await user.selectOptions(select, "100");
    expect(screen.getByText("of 13")).toBeInTheDocument();
  });

  it("offers no size select without a size handler, and holds still while loading", () => {
    render(<Pager page={1} pageSize={10} total={30} onPage={() => {}} loading />);
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(screen.getByRole("button", { name: "Next page" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
    expect(screen.getByText("Loading…")).toBeInTheDocument();
  });

  it("reads an empty list as nothing on screen", () => {
    render(<Pager page={0} pageSize={10} total={0} onPage={() => {}} />);
    expect(screen.getByText("0 of 0")).toBeInTheDocument();
    expect(screen.getByText("of 1")).toBeInTheDocument();
  });
});
