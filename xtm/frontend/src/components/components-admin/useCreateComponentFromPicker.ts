import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useConfirm } from "@agile-suite/core";
import { CreateComponent, errMsg } from "../../api";
import { useCapabilities } from "../../features";
import { keys } from "../../queries/keys";

// useCreateComponentFromPicker backs a component picker's inline create: ask,
// create in Jira, refresh the component lists. Without component admin the
// picker gets no onCreate and offers no create row.
export function useCreateComponentFromPicker(profileId: string, projectKey: string) {
  const caps = useCapabilities(profileId);
  const { confirm } = useConfirm();
  const qc = useQueryClient();
  const [error, setError] = useState("");

  async function onCreate(name: string): Promise<boolean> {
    setError("");
    const ok = await confirm({
      title: "Create component",
      message: `Create component "${name}" in ${projectKey}?`,
      confirmLabel: "Create",
    });
    if (!ok) return false;
    try {
      await CreateComponent(profileId, { name, description: "", leadUserName: "", assigneeType: "PROJECT_DEFAULT" });
      await qc.invalidateQueries({ queryKey: keys.components(profileId) });
      return true;
    } catch (e) {
      setError(errMsg(e));
      return false;
    }
  }

  return {
    onCreate: caps.supportsComponentAdmin ? onCreate : undefined,
    error,
    clearError: () => setError(""),
  };
}
