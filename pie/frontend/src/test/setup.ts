import "@testing-library/jest-dom/vitest";

import { afterEach } from "vitest";
import { cleanup, configure } from "@testing-library/react";

// findBy* waits 1000ms by default. With six test files in parallel, App's
// first render (StartupError, then ListProfiles, then the profile's queries)
// can miss that window and fail a test that passes alone, as TAM's did.
configure({ asyncUtilTimeout: 5000 });

// Vitest globals are off, so Testing Library cannot register this itself.
afterEach(cleanup);
