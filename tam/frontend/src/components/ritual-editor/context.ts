import { createContext } from "react";

// The profile a node view reads the cache for. Node views render through
// portals under EditorContent, so React context reaches them.
export const RitualEditorContext = createContext<{ profileId: string }>({ profileId: "" });
