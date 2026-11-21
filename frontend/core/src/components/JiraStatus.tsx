// The three colours a Jira status can take, after its status category.
export type StatusCategory = "todo" | "active" | "done";

// statusCategory picks the colour from Jira's status category when the row
// carries one ("new", "indeterminate", "done"), because a status name is the
// instance's own word and "Erledigt" is as done as "Done". Without one it
// guesses from the name: XTM stores no category, and a TAM row synced before
// schema 19 has none.
export function statusCategory(status: string, category?: string): StatusCategory {
  switch (category) {
    case "done":
      return "done";
    case "indeterminate":
      return "active";
    case "new":
      return "todo";
  }
  const s = status.toLowerCase();
  if (s === "done" || s === "closed" || s === "resolved") return "done";
  if (s.includes("progress") || s === "in review") return "active";
  return "todo";
}

interface Props {
  status: string;
  category?: string;
  // Extra classes for a placement rule, such as a smaller chip in a list.
  className?: string;
}

// JiraStatus is the one chip both apps draw a Jira status in (#539). TAM had
// a chip coloured by category and XTM one sky-blue pill for every status.
export function JiraStatus({ status, category, className }: Props) {
  const tone = statusCategory(status, category);
  return (
    <span className={`chip-status chip-status-${tone}${className ? ` ${className}` : ""}`} title={status}>
      {status}
    </span>
  );
}
