import { useConfirm } from "@agile-suite/core";
import type { saved } from "../wailsjs/go/models";

// useReplaceJql opens a saved query or a Jira filter, both of which replace the
// JQL field. JQL that differs from the open query's saved text, or any JQL
// when no query is open, is asked about first (#545). Cancelling changes
// nothing, so the typed JQL and the selected query both stay.
export function useReplaceJql({
  jql,
  setJql,
  queries,
  queryId,
  openQuery,
}: {
  jql: string;
  setJql: (jql: string) => void;
  queries: saved.Query[];
  queryId: number;
  openQuery: (id: number) => Promise<void>;
}) {
  const { confirm } = useConfirm();

  async function mayReplace(next: string) {
    const savedText = queries.find((q) => q.id === queryId)?.jql ?? "";
    if (jql.trim() === savedText.trim() || jql.trim() === next.trim()) return true;
    return confirm({
      title: "Replace the JQL?",
      message: "The JQL you typed is not saved. Opening this puts other JQL in its place.",
      confirmLabel: "Replace",
      danger: true,
    });
  }

  return {
    // "Unsaved query" keeps the JQL as it is, so only a saved query can ask.
    openSaved: async (id: number) => {
      if (id !== 0 && !(await mayReplace(queries.find((q) => q.id === id)?.jql ?? ""))) return;
      await openQuery(id);
    },
    pickFilter: async (id: string) => {
      // A filter reference follows the filter as its owner edits it.
      const next = `filter = ${id}`;
      if (!(await mayReplace(next))) return;
      setJql(next);
      await openQuery(0);
    },
  };
}
