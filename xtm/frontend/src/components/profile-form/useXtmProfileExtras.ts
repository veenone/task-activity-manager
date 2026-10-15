import { useState } from "react";
import type { CredentialOverride } from "@agile-suite/core";
import type { Connection, Profile } from "../../api";

// useXtmProfileExtras holds what XTM's profile form adds to core's: the
// backend (Xray or Kiwi), a connection's role, the Xray bug-filing fields and
// cross-project sources, and Kiwi's username and password.
export function useXtmProfileExtras(profile: Profile | undefined, connection: Connection | undefined) {
  const seed = profile ?? connection;
  // New profiles default to Xray, so behaviour only changes when the user
  // picks Kiwi.
  const [backend, setBackend] = useState(seed?.backend === "kiwi" ? "kiwi" : "xray");
  const isKiwi = backend === "kiwi";
  // A connection's role: "source", "target" or "both". New connections are
  // targets, the common bridge case beside the workspace's primary one.
  const [role, setRole] = useState(connection?.role || "target");
  const [crossProjectSources, setCrossProjectSources] = useState(profile?.crossProjectSources ?? "");
  const [bugIssueType, setBugIssueType] = useState(seed?.bugIssueType ?? "");
  const [bugProjectMode, setBugProjectMode] = useState(seed?.bugProjectMode || "test");
  const [bugProjectKey, setBugProjectKey] = useState(seed?.bugProjectKey ?? "");
  // Kiwi signs in with a username and password, sent as "username:password"
  // in the token argument (internal/backend/kiwi splits on the first ":").
  // Two fields, so nobody has to type the combined string.
  const [kiwiUsername, setKiwiUsername] = useState("");
  const [kiwiPassword, setKiwiPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);

  const user = kiwiUsername.trim();
  const pass = kiwiPassword.trim();
  const complete = user !== "" && pass !== "";
  // One half filled is never a credential. Gated on Kiwi, so halves left over
  // from a backend round trip cannot hold the Xray form back.
  const kiwiCredInvalid = isKiwi && (user !== "" || pass !== "") && !complete;
  // Kiwi's credential replaces core's token; Xray uses core's.
  const credential: CredentialOverride | undefined = isKiwi
    ? { value: complete ? `${user}:${pass}` : "", entered: complete, invalid: kiwiCredInvalid }
    : undefined;

  // The bug project key is only sent for a dedicated bug project.
  const bugProjKey = bugProjectMode === "dedicated" ? bugProjectKey.trim().toUpperCase() : "";

  // copyFrom takes the Xray settings of the profile a new one starts from.
  function copyFrom(src: Profile) {
    setCrossProjectSources(src.crossProjectSources ?? "");
    setBugIssueType(src.bugIssueType ?? "");
    setBugProjectMode(src.bugProjectMode || "test");
    setBugProjectKey(src.bugProjectKey ?? "");
  }

  return {
    backend, setBackend, isKiwi, role, setRole, crossProjectSources, setCrossProjectSources,
    bugIssueType, setBugIssueType, bugProjectMode, setBugProjectMode, bugProjectKey, setBugProjectKey, bugProjKey,
    kiwiUsername, setKiwiUsername, kiwiPassword, setKiwiPassword, showPassword, setShowPassword,
    kiwiCredInvalid, credential, copyFrom,
  };
}

export type XtmProfileExtras = ReturnType<typeof useXtmProfileExtras>;
