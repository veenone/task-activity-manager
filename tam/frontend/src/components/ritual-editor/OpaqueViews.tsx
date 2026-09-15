import { NodeViewWrapper } from "@tiptap/react";
// NodeViewProps is declared by @tiptap/core and only re-exported from
// @tiptap/react under the name ReactNodeViewProps (an intersection with a
// ref type this component does not need), so it is imported from its home
// package directly rather than through @tiptap/react.
import type { NodeViewProps } from "@tiptap/core";
import { MacroPreview } from "./MacroPreview";

export function opaqueTitle(label: string): string {
  return label === "jira" ? "Jira issues" : `Confluence: ${label || "content"}`;
}

export function OpaqueBlockView({ node, selected }: NodeViewProps) {
  const label = String(node.attrs.label ?? "");
  return (
    <NodeViewWrapper className={`ritual-opaque${selected ? " is-selected" : ""}`} contentEditable={false} data-drag-handle>
      <span className="ritual-opaque-label">{opaqueTitle(label)}</span>
      {label === "jira"
        ? <MacroPreview xml={String(node.attrs.xml ?? "")} />
        : <span className="muted small">Edit this in Confluence. TAM keeps it exactly as it is.</span>}
    </NodeViewWrapper>
  );
}

export function OpaqueInlineView({ node }: NodeViewProps) {
  return (
    <NodeViewWrapper as="span" className="ritual-opaque-inline" contentEditable={false} title="Kept exactly as it is. Edit it in Confluence.">
      {String(node.attrs.label || "content")}
    </NodeViewWrapper>
  );
}
