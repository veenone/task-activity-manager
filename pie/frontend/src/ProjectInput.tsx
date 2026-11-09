import { useEffect, useId, useRef, useState } from "react";

// listId is a datalist id safe for a CSS selector; useId's colons are not.
const useListId = () => `pie-projects-${useId().replace(/:/g, "")}`;

// resolve finds the project a text names, by key or by its label, ignoring
// case; "" when it names none.
function resolve(text: string, projects: string[], label: (p: string) => string): string {
  const t = text.trim().toLowerCase();
  if (t === "") return "";
  return projects.find((p) => p.toLowerCase() === t || label(p).toLowerCase() === t) ?? "";
}

function Suggestions({ id, projects, label }: { id: string; projects: string[]; label: (p: string) => string }) {
  return (
    <datalist id={id}>
      {projects.map((p) => (
        <option key={p} value={p}>
          {label(p)}
        </option>
      ))}
    </datalist>
  );
}

// ProjectInput is a project filter you can type into. The browser's own
// suggestion list narrows by key or name as you type, so a run reaching
// dozens of projects needs no scrolling. value is a project key, "" for all.
export function ProjectInput({
  label,
  projects,
  projectLabel,
  value,
  onChange,
  allText = "All projects",
}: {
  label: string;
  projects: string[];
  projectLabel: (p: string) => string;
  value: string;
  onChange: (project: string) => void;
  allText?: string;
}) {
  const id = useListId();
  const [text, setText] = useState(value);
  // emitted is the last value this box sent. A value that differs from it was
  // set from outside, and replaces the text; any other change of render, such
  // as a new projects array, leaves what the analyst is typing alone.
  const emitted = useRef(value);
  useEffect(() => {
    if (value !== emitted.current) {
      emitted.current = value;
      setText(value);
    }
  }, [value]);
  const unknown = text.trim() !== "" && resolve(text, projects, projectLabel) === "";
  return (
    <>
      <label>
        {label}
        <input
          className="detail-input pie-filter-select"
          type="search"
          list={id}
          value={text}
          placeholder={allText}
          onChange={(e) => {
            const p = resolve(e.target.value, projects, projectLabel);
            setText(e.target.value);
            emitted.current = p;
            onChange(p);
          }}
        />
      </label>
      <Suggestions id={id} projects={projects} label={projectLabel} />
      <span role="status" className="muted">
        {unknown ? "No project matches" : ""}
      </span>
    </>
  );
}

// ProjectsChooser picks several projects: type to add one, remove it from
// its chip. None chosen means every project.
export function ProjectsChooser({
  projects,
  projectLabel,
  chosen,
  onChange,
}: {
  projects: string[];
  projectLabel: (p: string) => string;
  chosen: string[];
  onChange: (projects: string[]) => void;
}) {
  const id = useListId();
  const [text, setText] = useState("");
  const left = projects.filter((p) => !chosen.includes(p));
  const add = (t: string) => {
    const p = resolve(t, left, projectLabel);
    if (!p) return false;
    onChange([...chosen, p]);
    setText("");
    return true;
  };
  return (
    <div className="pie-chooser">
      <label>
        Add a project
        <input
          className="detail-input"
          type="search"
          list={id}
          value={text}
          placeholder="Every project"
          onChange={(e) => {
            // A pick from the suggestion list replaces the text in one go.
            const picked = (e.nativeEvent as InputEvent).inputType === "insertReplacementText";
            if (!(picked && add(e.target.value))) setText(e.target.value);
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              add(text);
            }
          }}
        />
      </label>
      <Suggestions id={id} projects={left} label={projectLabel} />
      <ul className="pie-chips" aria-label="Chosen projects">
        {chosen.map((p) => (
          <li key={p}>
            {projectLabel(p)}
            <button
              type="button"
              className="btn btn-ghost"
              aria-label={`Remove ${projectLabel(p)}`}
              onClick={() => onChange(chosen.filter((c) => c !== p))}
            >
              ×
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
