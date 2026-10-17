import type { PendingChange, Folder } from "../../api";

// Where the reviewer name is remembered across tests.
export const REVIEWER_KEY = "xtm.reviewer";

export interface TestDetailProps {
  testKey: string;
  version: number;
  pendingForTest: PendingChange[];
  folders: Folder[];
  // The active profile's Jira base URL, used to link the test key to its real
  // Jira issue (RND_P_4TFINT_05-211). Optional / empty hides the link.
  jiraUrl?: string;
  onClose: () => void;
  onEdited: () => void;
  // Called with the new temp key after cloning this test into a fresh draft.
  // Optional: omitted where the panel is a read-only slide-over, which hides
  // the Clone action.
  onCloned?: (tempKey: string) => void;
  // When true, all editing controls are hidden and every mutating handler
  // short-circuits. Read display (fields, steps, preconditions, requirements,
  // custom fields, bugs, run history) remains fully functional.
  readOnly?: boolean;
  // The Test Execution this test is being looked at inside, when the panel
  // was opened from one. An execution can define its own dataset for a test,
  // and that is what runs there (#277 W3).
  containerKey?: string;
}

export type EditableField =
  | "summary"
  | "description"
  | "priority"
  | "labels"
  | "cucumber_scenario"
  | "cucumber_type"
  | "generic_definition";

// EXEC_TYPE_OPTIONS is the fixed Xray Test Type (execution type) vocabulary.
export const EXEC_TYPE_OPTIONS = ["Manual", "Automated", "Generic", "Cucumber"];
