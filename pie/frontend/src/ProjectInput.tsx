import { useEffect, useId, useRef, useState } from "react";

// resolve finds the project a text names, by key or by its label, ignoring
// case; "" when it names none.
function resolve(text: string, projects: string[], label: (p: string) => string): string {
  const t = text.trim().toLowerCase();
  if (t === "") return "";
  return projects.find((p) => p.toLowerCase() === t || label(p).toLowerCase() === t) ?? "";
}

// Combo is a text box with PIE's own suggestion list (#454). The browser's
// datalist popup ignored the app's theme. The list holds the projects whose
// key or name contains the text, opens on focus, and is worked with the
// arrow keys, Enter and Escape, as an ARIA combobox.
export function Combo({
  name,
  text,
  onText,
  projects,
  label,
  onPick,
  onEnter,
  placeholder,
  className,
}: {
  // The box's accessible name. The open list sits inside the box's <label>,
  // whose text would otherwise take in every suggestion.
  name: string;
  text: string;
  onText: (text: string) => void;
  projects: string[];
  label: (p: string) => string;
  onPick: (project: string) => void;
  // Enter with no suggestion highlighted.
  onEnter?: () => void;
  placeholder: string;
  className: string;
}) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(-1);
  // typed is whether the text was typed since the box took focus or a pick.
  // A box opened on a chosen project lists them all, so the others are still
  // on offer.
  const [typed, setTyped] = useState(false);
  const t = text.trim().toLowerCase();
  const matches =
    t === "" || !typed
      ? projects
      : projects.filter((p) => p.toLowerCase().includes(t) || label(p).toLowerCase().includes(t));
  const shown = open && matches.length > 0;
  const pick = (p: string) => {
    onPick(p);
    setOpen(false);
    setActive(-1);
    setTyped(false);
  };
  return (
    <span className="pie-combo">
      <input
        className={className}
        type="search"
        role="combobox"
        aria-label={name}
        aria-expanded={shown}
        aria-controls={`${id}-list`}
        aria-autocomplete="list"
        aria-activedescendant={shown && active >= 0 ? `${id}-${active}` : undefined}
        value={text}
        placeholder={placeholder}
        onFocus={() => {
          setOpen(true);
          setTyped(false);
        }}
        onBlur={() => setOpen(false)}
        onChange={(e) => {
          onText(e.target.value);
          setOpen(true);
          setTyped(true);
          setActive(-1);
        }}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown" || e.key === "ArrowUp") {
            e.preventDefault();
            const step = e.key === "ArrowDown" ? 1 : -1;
            setOpen(true);
            setActive((a) => (matches.length ? (a + step + matches.length) % matches.length : -1));
          } else if (e.key === "Escape" && shown) {
            e.preventDefault();
            setOpen(false);
          } else if (e.key === "Enter") {
            if (shown && active >= 0 && matches[active]) {
              e.preventDefault();
              pick(matches[active]);
            } else if (onEnter) {
              e.preventDefault();
              onEnter();
            }
          }
        }}
      />
      {shown && (
        <ul id={`${id}-list`} role="listbox" className="pie-combo-list">
          {matches.map((p, i) => (
            <li
              key={p}
              id={`${id}-${i}`}
              role="option"
              aria-selected={i === active}
              className={i === active ? "pie-combo-active" : undefined}
              // Keeps the focus in the box, so the blur does not close the
              // list before the click lands.
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => pick(p)}
            >
              {label(p)}
            </li>
          ))}
        </ul>
      )}
    </span>
  );
}

// ProjectInput is a project filter you can type into. Its suggestion list
// narrows by key or name as you type, so a run reaching dozens of projects
// needs no scrolling. value is a project key, "" for all.
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
        <Combo
          name={label}
          className="detail-input pie-filter-select"
          text={text}
          onText={(t) => {
            const p = resolve(t, projects, projectLabel);
            setText(t);
            emitted.current = p;
            onChange(p);
          }}
          projects={projects}
          label={projectLabel}
          onPick={(p) => {
            setText(p);
            emitted.current = p;
            onChange(p);
          }}
          placeholder={allText}
        />
      </label>
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
  const [text, setText] = useState("");
  const left = projects.filter((p) => !chosen.includes(p));
  const add = (t: string) => {
    const p = resolve(t, left, projectLabel);
    if (!p) return;
    onChange([...chosen, p]);
    setText("");
  };
  return (
    <div className="pie-chooser">
      <label>
        Add a project
        <Combo
          name="Add a project"
          className="detail-input"
          text={text}
          onText={setText}
          projects={left}
          label={projectLabel}
          onPick={(p) => add(p)}
          onEnter={() => add(text)}
          placeholder="Every project"
        />
      </label>
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
