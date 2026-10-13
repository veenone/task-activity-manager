// ASSIGNEE_TYPES are the default assignee values Jira takes for a component,
// with the wording the form and the import template use for each.
export const ASSIGNEE_TYPES = [
  { value: "PROJECT_DEFAULT", label: "Project default" },
  { value: "COMPONENT_LEAD", label: "Component lead" },
  { value: "PROJECT_LEAD", label: "Project lead" },
  { value: "UNASSIGNED", label: "Unassigned" },
];

export function assigneeLabel(value: string): string {
  return ASSIGNEE_TYPES.find((t) => t.value === value)?.label ?? value;
}
