import { usePriorities } from "../queries/people";

interface Props {
  profileId: string;
  value: string;
  onChange: (priority: string) => void;
  id: string;
  disabled?: boolean;
  // What an empty value means to this form: a create leaves Jira's default,
  // an edit leaves the issue's current priority alone.
  emptyLabel: string;
}

// PriorityPicker offers the instance's own priority names instead of asking
// the user to type one from memory. A free-text field could only be validated
// by Jira at Commit, which is the wrong place to learn that "Urgent" is not a
// priority on this instance.
//
// When the list cannot be read the control degrades to the text input it used
// to be, rather than blocking the form: the same shape the create dialog uses
// for a failed create-meta read.
export function PriorityPicker({ profileId, value, onChange, id, disabled, emptyLabel }: Props) {
  const priorities = usePriorities(profileId);

  if (priorities.isError) {
    return (
      <span className="edit-cell">
        <input
          id={id}
          className="detail-input"
          type="text"
          disabled={disabled}
          placeholder={emptyLabel}
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
        <span className="muted small">
          The priority list could not be read ({priorities.error.message}); type one instead.
        </span>
      </span>
    );
  }

  // A value the list does not carry still has to be selectable, or opening an
  // issue whose priority was renamed in Jira would silently change it.
  const options = priorities.data ?? [];
  const unlisted = value !== "" && !options.includes(value);

  return (
    <select
      id={id}
      className="detail-input"
      disabled={disabled || priorities.isPending}
      value={value}
      onChange={(e) => onChange(e.target.value)}
    >
      <option value="">{priorities.isPending ? "Loading" : emptyLabel}</option>
      {unlisted && <option value={value}>{value}</option>}
      {options.map((p) => (
        <option key={p} value={p}>{p}</option>
      ))}
    </select>
  );
}
