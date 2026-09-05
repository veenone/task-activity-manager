import type { ProfileBackend } from "@agile-suite/core";
import {
  ListProfiles,
  GetSettings,
  SetTheme,
  SetDefaultProfile,
} from "./api";
import type { Profile, Settings } from "./api";

// The adapter the shared ProfileProvider talks through. Bindings are wrapped
// in arrow functions so tests can mock ./api without touching this file.
export const profileBackend: ProfileBackend<Profile, Settings> = {
  listProfiles: () => ListProfiles(),
  getSettings: () => GetSettings(),
  setTheme: (theme) => SetTheme(theme),
  setDefaultProfile: (id) => SetDefaultProfile(id),
};
