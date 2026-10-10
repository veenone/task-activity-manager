import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FolderTree } from "./FolderTree";
import type { Folder } from "../api";

function folder(over: Partial<Folder> = {}): Folder {
  return {
    id: "1",
    parentId: "",
    name: "Regression",
    testCount: 3,
    totalTestCount: 3,
    ...over,
  };
}

describe("FolderTree", () => {
  it("still offers All tests when the repository has no folders", async () => {
    const onSelect = vi.fn();
    render(<FolderTree folders={[]} selected="" onSelect={onSelect} totalTests={10} />);

    // The whole point of the row: a repository whose tests all sit outside a
    // folder is reachable through it and nothing else.
    expect(screen.getByText("All tests")).toBeInTheDocument();
    expect(screen.getByText("10")).toBeInTheDocument();

    await userEvent.click(screen.getByText("All tests"));
    expect(onSelect).toHaveBeenCalledWith("");
  });

  it("counts every test, not only the ones filed in a folder", () => {
    render(
      <FolderTree
        folders={[folder({ id: "1", name: "Regression", testCount: 4, totalTestCount: 4 })]}
        selected=""
        onSelect={() => {}}
        totalTests={10}
      />,
    );

    // Four tests are in Regression and six are loose, so the root badge reads
    // ten rather than the four the folders account for.
    expect(screen.getByText("10")).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
  });

  it("falls back to summing the root folders when no total is given", () => {
    render(
      <FolderTree
        folders={[
          folder({ id: "1", name: "Regression", totalTestCount: 4 }),
          folder({ id: "2", name: "Smoke", totalTestCount: 3 }),
        ]}
        selected=""
        onSelect={() => {}}
      />,
    );

    expect(screen.getByText("7")).toBeInTheDocument();
  });
});

describe("FolderTree keyboard", () => {
  const tree = [
    folder({ id: "1", parentId: "", name: "Regression" }),
    folder({ id: "2", parentId: "1", name: "Login" }),
  ];

  it("selects a folder with Enter, as a click does", async () => {
    const onSelect = vi.fn();
    render(<FolderTree folders={tree} selected="" onSelect={onSelect} readOnly />);

    screen.getByRole("treeitem", { name: /Regression/ }).focus();
    await userEvent.keyboard("{Enter}");

    expect(onSelect).toHaveBeenCalledWith("1");
  });

  it("collapses with ArrowLeft and expands with ArrowRight", async () => {
    render(<FolderTree folders={tree} selected="" onSelect={() => {}} readOnly />);

    screen.getByRole("treeitem", { name: /Regression/ }).focus();
    await userEvent.keyboard("{ArrowLeft}");
    expect(screen.queryByText("Login")).toBeNull();

    await userEvent.keyboard("{ArrowRight}");
    expect(screen.getByText("Login")).toBeInTheDocument();
  });

  it("moves to the next visible folder with ArrowDown", async () => {
    render(<FolderTree folders={tree} selected="" onSelect={() => {}} readOnly />);

    screen.getByRole("treeitem", { name: /Regression/ }).focus();
    await userEvent.keyboard("{ArrowDown}");

    expect(document.activeElement).toHaveTextContent("Login");
  });
});
