import "@testing-library/jest-dom/vitest";

import { afterEach, vi } from "vitest";
import { cleanup, configure } from "@testing-library/react";

// findBy* waits 1000ms by default. With six test files in parallel, App's
// first render (StartupError, then ListProfiles, then the profile's queries)
// can miss that window and fail a test that passes alone, as TAM's did.
configure({ asyncUtilTimeout: 5000 });

// Vitest globals are off, so Testing Library cannot register this itself.
afterEach(cleanup);

// The Wails runtime is not there in jsdom. Tests that drive the menu mock it
// with their own listeners.
vi.mock("../../wailsjs/runtime/runtime", () => ({ EventsOn: () => () => undefined, BrowserOpenURL: () => undefined }));
