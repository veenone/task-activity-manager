import { useState } from "react";

// drafts holds text typed but not yet sent, by a key naming the field and its
// case. CaseDetail remounts for every case and the Cases view unmounts with its
// tab, so state kept in the component would lose the text on either (#543).
// The drafts last for the session, like any unsent text.
const drafts = new Map<string, string>();

// useDraft is useState for a draft that outlives its component. The key is
// read once, when the component mounts.
export function useDraft(key: string): [string, (text: string) => void] {
  const [text, setText] = useState(() => drafts.get(key) ?? "");
  const set = (next: string) => {
    setText(next);
    if (next) drafts.set(key, next);
    else drafts.delete(key);
  };
  return [text, set];
}
