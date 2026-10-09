import { TokenPicker, validateLabel } from "./TokenPicker";
import { useLabels } from "../queries/app";

interface LabelsFieldProps {
  profileId: string;
  value: string;
  onChange: (v: string) => void;
  onSave: () => void;
  readOnly: boolean;
}

// LabelsField is the test detail's label control. It keeps the space-joined
// string TestDetail stores, so saveField and the dirty marker are unchanged.
export function LabelsField({ profileId, value, onChange, onSave, readOnly }: LabelsFieldProps) {
  const { data: buckets = [] } = useLabels(profileId);
  if (readOnly) return <span>{value || "—"}</span>;
  return (
    <TokenPicker
      label="Labels"
      value={value.split(/\s+/).filter(Boolean)}
      onChange={(next) => onChange(next.join(" "))}
      onBlur={onSave}
      suggestions={buckets.map((b) => b.label)}
      allowCreate
      validate={validateLabel}
      placeholder="Type to search or create"
    />
  );
}
