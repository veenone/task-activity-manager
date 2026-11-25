import { useState } from "react";
import { errMsg } from "@agile-suite/core";
import { ExportDependencies } from "../api";
import { DEP_LAYOUT, type GraphExport } from "../lib/dependencyGraph";

// useDependencyExport writes the Dependencies view to a workbook (#612): the
// scope's rows from the backend, and a picture of its graph drawn off screen.
// A picture that cannot be drawn does not cost the user the workbook: it is
// written without one and the note says so.
export function useDependencyExport(profileId: string) {
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState("");
  const [note, setNote] = useState("");
  const [error, setError] = useState("");

  async function run(out: GraphExport) {
    setBusy(true);
    setSaved("");
    setNote("");
    setError("");
    let image = "";
    if (out.nodes.length) {
      try {
        // G6 is loaded with the view, not with the app.
        const { graphImage } = await import("@agile-suite/core/src/g6/G6Canvas");
        image = await graphImage(out.nodes, out.edges, out.scope.focus || null, DEP_LAYOUT);
      } catch (e) {
        setNote(`without the graph picture: ${errMsg(e)}`);
      }
    }
    try {
      // An empty path is a cancelled dialog: nothing to announce.
      setSaved(await ExportDependencies(profileId, out.scope, image));
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  return { busy, saved, note, error, run };
}
