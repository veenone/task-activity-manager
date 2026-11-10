import { expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { compare, useSort } from "./sort";

type Row = { key: string; n: number };
const data: Row[] = [
  { key: "A-10", n: 2 },
  { key: "A-2", n: 30 },
  { key: "A-1", n: 2 },
];

function Table() {
  const { sorted, th } = useSort(data, { key: (r) => r.key, n: (r) => r.n });
  return (
    <table aria-label="T">
      <thead>
        <tr>
          {th("key", "Key")}
          {th("n", "Count")}
        </tr>
      </thead>
      <tbody>
        {sorted.map((r) => (
          <tr key={r.key}>
            <td>{r.key}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

const order = () => within(screen.getByRole("table")).getAllByRole("cell").map((c) => c.textContent);

it("keeps the given order until a header is clicked, then sorts and reverses", async () => {
  render(<Table />);
  const user = userEvent.setup();
  expect(order()).toEqual(["A-10", "A-2", "A-1"]);
  expect(screen.getByRole("columnheader", { name: /Key/ })).toHaveAttribute("aria-sort", "none");

  await user.click(screen.getByRole("button", { name: "Sort by Key" }));
  // Numbers inside text sort as numbers.
  expect(order()).toEqual(["A-1", "A-2", "A-10"]);
  expect(screen.getByRole("columnheader", { name: /Key/ })).toHaveAttribute("aria-sort", "ascending");

  await user.click(screen.getByRole("button", { name: "Sort by Key" }));
  expect(order()).toEqual(["A-10", "A-2", "A-1"]);
  expect(screen.getByRole("columnheader", { name: /Key/ })).toHaveAttribute("aria-sort", "descending");

  // A number column sorts by value; ties keep their order.
  await user.click(screen.getByRole("button", { name: "Sort by Count" }));
  expect(order()).toEqual(["A-10", "A-1", "A-2"]);
  expect(screen.getByRole("columnheader", { name: /Key/ })).toHaveAttribute("aria-sort", "none");
});

it("compares text without case and numbers by value, and puts blanks last", () => {
  expect(["b", "", "A", "a10", "a9"].sort(compare)).toEqual(["A", "a9", "a10", "b", ""]);
  expect([10, 9, 100].sort(compare)).toEqual([9, 10, 100]);
});
