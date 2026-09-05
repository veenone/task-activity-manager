import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
} from "react";
import type { Dispatch, ReactNode, SetStateAction } from "react";
import { applyTheme } from "../lib/theme";
import { errMsg } from "../lib/errMsg";

// The calls the provider needs from an app's backend. Each app builds one
// from its own generated Wails bindings, so this package never imports them.
export interface ProfileBackend<
  P extends { id: string },
  S extends { defaultProfileId?: string; theme?: string },
> {
  listProfiles: () => Promise<P[]>;
  getSettings: () => Promise<S>;
  setTheme: (theme: string) => Promise<void>;
  setDefaultProfile: (id: string) => Promise<void>;
}

export interface ProfileState<
  P extends { id: string },
  S extends { defaultProfileId?: string; theme?: string },
> {
  profiles: P[];
  activeId: string;
  defaultProfileId: string;
  theme: string;
  loading: boolean;
  // The message from the last failed reload, empty when the last one worked.
  error: string;
  activeProfile: P | undefined;
  setActiveId: Dispatch<SetStateAction<string>>;
  // Applies the theme at once, then persists it.
  setTheme: (next: string) => Promise<void>;
  // Makes id the launch default, or clears the default if id already is.
  setDefault: (id: string) => Promise<void>;
  // Loads profiles and settings, applies the theme, picks the launch profile,
  // and returns the settings so the caller can read anything app-specific.
  reload: () => Promise<S | null>;
}

// One context object serves every app; the hook casts to the app's own
// profile and settings shapes. The cast is safe because the provider is the
// only writer and it is typed by the backend it was given.
const ProfileContext = createContext<ProfileState<{ id: string }, object> | null>(null);

export function useProfile<
  P extends { id: string },
  S extends { defaultProfileId?: string; theme?: string },
>(): ProfileState<P, S> {
  const ctx = useContext(ProfileContext);
  if (!ctx) {
    throw new Error("useProfile must be used within a ProfileProvider");
  }
  return ctx as unknown as ProfileState<P, S>;
}

export function ProfileProvider<
  P extends { id: string },
  S extends { defaultProfileId?: string; theme?: string },
>({ backend, children }: { backend: ProfileBackend<P, S>; children: ReactNode }) {
  const [profiles, setProfiles] = useState<P[]>([]);
  const [activeId, setActiveId] = useState("");
  const [defaultProfileId, setDefaultProfileId] = useState("");
  const [theme, setThemeState] = useState("light");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const setTheme = useCallback(
    async (next: string) => {
      setThemeState(next);
      applyTheme(next);
      try {
        await backend.setTheme(next);
      } catch (e) {
        console.error("set theme:", errMsg(e));
      }
    },
    [backend],
  );

  const setDefault = useCallback(
    async (id: string) => {
      const next = defaultProfileId === id ? "" : id;
      try {
        await backend.setDefaultProfile(next);
        setDefaultProfileId(next);
      } catch (e) {
        console.error("set default profile:", errMsg(e));
      }
    },
    [backend, defaultProfileId],
  );

  const reload = useCallback(async (): Promise<S | null> => {
    setLoading(true);
    setError("");
    try {
      const [ps, s] = await Promise.all([
        backend.listProfiles(),
        backend.getSettings(),
      ]);
      setProfiles(ps);
      setDefaultProfileId(s.defaultProfileId ?? "");
      const t = s.theme || "light";
      setThemeState(t);
      applyTheme(t);
      if (ps.length > 0) {
        const def =
          s.defaultProfileId && ps.some((p) => p.id === s.defaultProfileId)
            ? s.defaultProfileId
            : ps[0].id;
        setActiveId(def);
      } else {
        setActiveId("");
      }
      return s;
    } catch (e) {
      const msg = errMsg(e);
      console.error("load profiles:", msg);
      setError(msg);
      return null;
    } finally {
      setLoading(false);
    }
  }, [backend]);

  const activeProfile = useMemo(
    () => profiles.find((p) => p.id === activeId),
    [profiles, activeId],
  );

  const value = useMemo<ProfileState<P, S>>(
    () => ({
      profiles,
      activeId,
      defaultProfileId,
      theme,
      loading,
      error,
      activeProfile,
      setActiveId,
      setTheme,
      setDefault,
      reload,
    }),
    [
      profiles,
      activeId,
      defaultProfileId,
      theme,
      loading,
      error,
      activeProfile,
      setTheme,
      setDefault,
      reload,
    ],
  );

  return (
    <ProfileContext.Provider
      value={value as unknown as ProfileState<{ id: string }, object>}
    >
      {children}
    </ProfileContext.Provider>
  );
}
