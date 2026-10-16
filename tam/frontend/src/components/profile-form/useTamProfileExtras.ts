import { useCallback, useEffect, useState } from "react";
import { GetConfluenceConfig, GetProfileSetting, GetSettings, SetConfluenceConfig, SetProfileSetting } from "../../api";
import type { Profile } from "../../api";
import { readRootPageInput } from "../../lib/confluenceRoot";

// REQUIREMENT_TYPE_KEY is the per-profile setting holding the Jira issue type
// name TAM syncs as a requirement. It lives in tam.db, not on the shared
// profile row, so it is read and written separately from the profile itself.
const REQUIREMENT_TYPE_KEY = "requirement_issue_type";

// DETAIL_CACHE_KEY is the per-profile setting holding how long a cached issue
// detail is served before TAM asks Jira again. It lives in tam.db beside the
// requirement type and is read and written the same way.
const DETAIL_CACHE_KEY = "detail_cache_minutes";

export const DETAIL_MINUTES_NOT_A_NUMBER =
  "Minutes is a whole number, such as 10. Use 0 to keep issue details cached until a Refresh.";

// detailMinutesError checks the freshness field. Blank is the default (ten
// minutes) and 0 is the offline setting, so the only thing refused is a value
// that is not a whole number of minutes.
function detailMinutesError(value: string): string {
  const v = value.trim();
  if (v === "" || /^\d+$/.test(v)) return "";
  return DETAIL_MINUTES_NOT_A_NUMBER;
}

// useTamProfileExtras holds what TAM's profile form adds to the shared one:
// the requirement issue type, the issue detail freshness, the Confluence
// Rituals settings, and the application setting that decides whether the
// project key is checked. All of it lives in tam.db, keyed by profile id, so
// it is loaded once the profile exists and written after the profile is.
export function useTamProfileExtras(profile: Profile | undefined) {
  const [requirementType, setRequirementType] = useState("");
  const [detailMinutes, setDetailMinutes] = useState("");
  const [confluenceURL, setConfluenceURL] = useState("");
  const [confluenceSpace, setConfluenceSpace] = useState("");
  const [confluenceRootPageID, setConfluenceRootPageID] = useState("");
  const [confluenceToken, setConfluenceToken] = useState("");
  const [confluenceLoaded, setConfluenceLoaded] = useState(false);
  // Where sprint reports are published. Both blank is what every profile had
  // before these existed, and it publishes where it always did: the rituals
  // space, under the sprint's own page or the rituals root.
  const [reportsSpace, setReportsSpace] = useState("");
  const [reportsRootPageID, setReportsRootPageID] = useState("");
  // Whether the project key is checked against Jira's documented shape, an
  // application setting (#99). It starts on, which is both the default and
  // what the field did before the setting existed, so a settings read that is
  // slow or fails leaves the check where it has always been.
  const [checkKeyPattern, setCheckKeyPattern] = useState(true);

  // The requirement type is only readable once the profile exists, so a new
  // profile starts blank and saves the field after its id comes back.
  useEffect(() => {
    if (!profile) return;
    let live = true;
    GetProfileSetting(profile.id, REQUIREMENT_TYPE_KEY)
      .then((v) => live && setRequirementType(v))
      .catch(() => {
        /* a missing setting reads as blank; the placeholder says the default */
      });
    GetProfileSetting(profile.id, DETAIL_CACHE_KEY)
      .then((v) => live && setDetailMinutes(v))
      .catch(() => {
        /* same: blank is the default freshness */
      });
    return () => {
      live = false;
    };
  }, [profile]);

  useEffect(() => {
    let live = true;
    GetSettings()
      .then((s) => live && setCheckKeyPattern(s.checkProjectKeyPattern !== false))
      .catch(() => { /* unreadable settings leave the check on, its default */ });
    return () => { live = false; };
  }, []);

  useEffect(() => {
    if (!profile) return;
    let live = true;
    Promise.resolve()
      .then(() => GetConfluenceConfig(profile.id))
      .then((c) => {
        if (!live) return;
        setConfluenceURL(c.baseURL);
        setConfluenceSpace(c.spaceKey);
        setConfluenceRootPageID(c.rootPageID);
        setReportsSpace(c.reportsSpaceKey ?? "");
        setReportsRootPageID(c.reportsRootPageID ?? "");
        setConfluenceLoaded(true);
      })
      .catch(() => { /* optional integration remains blank when unavailable */ });
    return () => { live = false; };
  }, [profile]);

  const minutesError = detailMinutesError(detailMinutes);
  // A Sync against a root page id that is not one answers 404 and offers to
  // create a root nobody needed, so the id is checked where it is typed.
  const rootInput = readRootPageInput(confluenceRootPageID, confluenceURL);
  const valid = rootInput.error === "" && minutesError === "";

  // fillDemo gives a new demo profile the sample Confluence settings.
  const fillDemo = useCallback(() => {
    setConfluenceURL((value) => value || "demo");
    setConfluenceSpace((value) => value || "DEMO");
    setConfluenceRootPageID((value) => value || "demo-root");
  }, []);

  // saveFor writes the extras for a profile the main write just saved.
  async function saveFor(id: string, isEdit: boolean, demo: boolean) {
    await SetProfileSetting(id, REQUIREMENT_TYPE_KEY, requirementType.trim());
    await SetProfileSetting(id, DETAIL_CACHE_KEY, detailMinutes.trim());
    const typedConfluence = confluenceURL.trim() || confluenceSpace.trim() || confluenceRootPageID.trim() || reportsSpace.trim();
    if ((isEdit && confluenceLoaded) || (!demo && typedConfluence) || confluenceToken.trim()) {
      await SetConfluenceConfig(id, {
        baseURL: confluenceURL.trim(),
        spaceKey: confluenceSpace.trim(),
        rootPageID: rootInput.id,
        reportsSpaceKey: reportsSpace.trim(),
        reportsRootPageID: reportsRootPageID.trim(),
      }, confluenceToken.trim());
      setConfluenceURL(confluenceURL.trim().replace(/\/+$/, ""));
      setConfluenceSpace(confluenceSpace.trim());
      setConfluenceRootPageID(rootInput.id);
      setReportsSpace(reportsSpace.trim());
      setConfluenceLoaded(true);
    }
  }

  return {
    requirementType, setRequirementType, detailMinutes, setDetailMinutes, minutesError,
    confluenceURL, setConfluenceURL, confluenceSpace, setConfluenceSpace,
    confluenceRootPageID, setConfluenceRootPageID, confluenceToken, setConfluenceToken, confluenceLoaded,
    reportsSpace, setReportsSpace, reportsRootPageID, setReportsRootPageID,
    rootInput, checkKeyPattern, valid, fillDemo, saveFor,
  };
}

export type TamProfileExtras = ReturnType<typeof useTamProfileExtras>;
