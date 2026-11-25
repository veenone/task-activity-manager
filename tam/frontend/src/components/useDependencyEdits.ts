import { useState } from "react";
import { errMsg, useConfirm, useNotice } from "@agile-suite/core";
import type { DependencyGraph, DependencyLink, Link, LinkDraft } from "../api";
import { useAddLink, useDiscardById } from "../queries/pending";
import { useRemoveLink, useRetypeLink } from "../queries/dependencies";
import { closesCycle, isBlocks } from "../lib/dependencyGraph";

// The pair a drag joined, waiting for the user to choose the link's type.
export interface Connecting {
  source: string;
  target: string;
}

// describeKey is the summary and Jira type of an issue the graph knows,
// cached or not, for the journal row a link edit writes.
function describeKey(g: DependencyGraph | undefined, key: string): { summary: string; type: string } {
  const iss = g?.issues.find((i) => i.key === key);
  if (iss) return { summary: iss.summary, type: iss.type ? iss.type[0].toUpperCase() + iss.type.slice(1) : "Issue" };
  const far = g?.external.find((e) => e.key === key);
  return { summary: far?.summary ?? "", type: far?.issueType || "Issue" };
}

// asLink is a graph link seen from the issue it is cached under, the shape
// RemoveLink and RetypeLink take.
export function asLink(g: DependencyGraph | undefined, l: DependencyLink): Link {
  const other = l.direction === "inward" ? l.outward : l.inward;
  const d = describeKey(g, other);
  return { id: l.id, direction: l.direction, type: l.type, key: other, summary: d.summary, issueType: d.type };
}

const CYCLE = (outward: string, inward: string) => ({
  title: "This link closes a loop",
  message: `${inward} already blocks ${outward}, directly or through other issues. Adding ${outward} blocks ${inward} makes each wait on the other. Add it anyway?`,
  confirmLabel: "Add the link",
  danger: false,
});

// useDependencyEdits is every link edit the Dependencies view makes (#612):
// a drag opens the type choice, and each edit is journaled for Commit. A
// Blocks link that would close a loop asks first.
export function useDependencyEdits(profileId: string, graph: DependencyGraph | undefined) {
  const [connecting, setConnecting] = useState<Connecting | null>(null);
  const add = useAddLink(profileId);
  const remove = useRemoveLink(profileId);
  const retype = useRetypeLink(profileId);
  const discard = useDiscardById(profileId);
  const { confirm } = useConfirm();
  const { notice } = useNotice();
  const links = graph?.links ?? [];
  const failed = (title: string) => (e: unknown) => void notice({ title, message: errMsg(e), tone: "error" });

  const connect = (source: string, target: string) => {
    if (source && target && source !== target) setConnecting({ source, target });
  };

  // addLink journals source's link of type and direction to target. It
  // resolves false when the user backs out of a loop, so the dialog stays.
  const addLink = async (type: string, direction: "outward" | "inward"): Promise<boolean> => {
    if (!connecting) return false;
    const { source, target } = connecting;
    const [outward, inward] = direction === "outward" ? [source, target] : [target, source];
    if (isBlocks(type) && closesCycle(links, outward, inward) && !(await confirm(CYCLE(outward, inward)))) return false;
    const d = describeKey(graph, target);
    const link: LinkDraft = { type, direction, toKey: target, toSummary: d.summary, toType: d.type };
    try {
      await add.mutateAsync({ key: source, link });
      setConnecting(null);
      return true;
    } catch (e) {
      failed("The link could not be added")(e);
      return false;
    }
  };

  const removeLink = (l: DependencyLink, onDone: () => void) =>
    remove.mutate({ key: l.owner, link: asLink(graph, l) }, { onSuccess: onDone, onError: failed("The link could not be removed") });

  const retypeLink = async (l: DependencyLink, type: string, onDone: () => void) => {
    const others = links.filter((o) => o !== l);
    if (isBlocks(type) && closesCycle(others, l.outward, l.inward) && !(await confirm(CYCLE(l.outward, l.inward)))) return;
    retype.mutate({ key: l.owner, link: asLink(graph, l), type }, { onSuccess: onDone, onError: failed("The link type could not be changed") });
  };

  const discardLink = (l: DependencyLink, onDone: () => void) =>
    discard.mutate({ id: l.pendingId, key: l.owner }, { onSuccess: onDone, onError: failed("Discard failed") });

  const busy = add.isPending || remove.isPending || retype.isPending || discard.isPending;
  return { connecting, connect, cancelConnect: () => setConnecting(null), addLink, removeLink, retypeLink, discardLink, busy };
}
