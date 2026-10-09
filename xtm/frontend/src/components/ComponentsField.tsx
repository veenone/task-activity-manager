import { TokenPicker } from "./TokenPicker";
import { useComponentOptions } from "../queries/components";
import { validateComponentName } from "../lib/components";
import { useCreateComponentFromPicker } from "./components-admin/useCreateComponentFromPicker";

interface Props {
  profileId: string;
  projectKey: string;
  value: string[];
  readOnly: boolean;
  onSave: (names: string[]) => void;
}

// ComponentsField is the test detail's component control. Each change saves
// at once, like LabelsField. Only the project's components are offered; with
// component admin an unknown name can be created after a confirm.
export function ComponentsField({ profileId, projectKey, value, readOnly, onSave }: Props) {
  const options = useComponentOptions(profileId, projectKey);
  const create = useCreateComponentFromPicker(profileId, projectKey);
  if (readOnly) return <span>{value.join(", ")}</span>;
  return (
    <>
      <TokenPicker
        label="Components"
        value={value}
        onChange={(next) => {
          create.clearError();
          onSave(next);
        }}
        suggestions={options.data ?? []}
        allowCreate={!!create.onCreate}
        onCreate={create.onCreate}
        validate={validateComponentName}
        separator={null}
        placeholder="Type to search"
      />
      {create.error && <p className="token-picker-error">{create.error}</p>}
    </>
  );
}
