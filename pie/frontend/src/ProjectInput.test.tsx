import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { ProjectInput, ProjectsChooser } from "./ProjectInput";

const projects = ["ALPHA", "BRAVO", "CHARLIE"];
const label = (p: string) => ({ ALPHA: "Card Platform (ALPHA)", BRAVO: "Reader (BRAVO)" })[p] ?? p;

function One({ onChange }: { onChange: (p: string) => void }) {
  const [value, setValue] = useState("");
  return (
    <ProjectInput
      label="Project"
      projects={projects}
      projectLabel={label}
      value={value}
      onChange={(p) => {
        setValue(p);
        onChange(p);
      }}
    />
  );
}

it("suggests every project by key and name, and applies a project once its key or name is typed", async () => {
  const onChange = vi.fn();
  render(<One onChange={onChange} />);
  const box = screen.getByLabelText("Project");
  const options = [...document.querySelectorAll(`#${box.getAttribute("list")} option`)].map((o) => [
    o.getAttribute("value"),
    o.textContent,
  ]);
  expect(options).toEqual([
    ["ALPHA", "Card Platform (ALPHA)"],
    ["BRAVO", "Reader (BRAVO)"],
    ["CHARLIE", "CHARLIE"],
  ]);
  const user = userEvent.setup();
  await user.type(box, "bravo");
  expect(onChange).toHaveBeenLastCalledWith("BRAVO");
  await user.clear(box);
  await user.type(box, "card platform (alpha)");
  expect(onChange).toHaveBeenLastCalledWith("ALPHA");
});

it("applies no project, and says so, for text that names none", async () => {
  const onChange = vi.fn();
  render(<One onChange={onChange} />);
  await userEvent.setup().type(screen.getByLabelText("Project"), "zulu");
  expect(onChange).toHaveBeenLastCalledWith("");
  expect(screen.getByText("No project matches")).toBeVisible();
});

function Many() {
  const [chosen, setChosen] = useState<string[]>([]);
  return <ProjectsChooser projects={projects} projectLabel={label} chosen={chosen} onChange={setChosen} />;
}

it("adds projects as chips by typing and removes them", async () => {
  render(<Many />);
  const user = userEvent.setup();
  const box = screen.getByLabelText("Add a project");
  await user.type(box, "alpha{Enter}");
  await user.type(box, "charlie{Enter}");
  expect(screen.getByRole("list", { name: "Chosen projects" })).toHaveTextContent("Card Platform (ALPHA)");
  expect(box).toHaveValue("");
  await user.click(screen.getByRole("button", { name: "Remove Card Platform (ALPHA)" }));
  expect(screen.getByRole("list", { name: "Chosen projects" })).not.toHaveTextContent("ALPHA");
  expect(screen.getByRole("list", { name: "Chosen projects" })).toHaveTextContent("CHARLIE");
});

it("keeps the text being typed while the value it names stays the same", async () => {
  function Busiest() {
    const [picked, setPicked] = useState("");
    const centre = picked || "ALPHA";
    return (
      <>
        <ProjectInput label="Centre" projects={[...projects]} projectLabel={label} value={picked} onChange={setPicked} allText="Busiest project" />
        <output>{centre}</output>
      </>
    );
  }
  render(<Busiest />);
  const user = userEvent.setup();
  const box = screen.getByLabelText("Centre");
  await user.type(box, "CHARL");
  expect(box).toHaveValue("CHARL");
  await user.type(box, "IE");
  expect(screen.queryByText("No project matches")).not.toBeInTheDocument();
  expect(document.querySelector("output")).toHaveTextContent("CHARLIE");
  await user.clear(box);
  expect(document.querySelector("output")).toHaveTextContent("ALPHA");
});

it("takes a value set from outside", async () => {
  function Outside() {
    const [value, setValue] = useState("");
    return (
      <>
        <ProjectInput label="Project" projects={projects} projectLabel={label} value={value} onChange={setValue} />
        <button type="button" onClick={() => setValue("BRAVO")}>
          Pick bravo
        </button>
      </>
    );
  }
  render(<Outside />);
  await userEvent.setup().click(screen.getByRole("button", { name: "Pick bravo" }));
  expect(screen.getByLabelText("Project")).toHaveValue("BRAVO");
});
