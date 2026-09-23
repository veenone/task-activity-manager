/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The suite renders rows whose wording is counted in civil days, so the
// runner's zone is an input to it. UTC is the one zone that cannot move a
// date across midnight relative to the stamps Jira sends. The workers
// inherit this, and an explicit TZ still wins, so a zone bug can still be
// hunted with TZ=Pacific/Kiritimati.
//
// Node on Windows ignores TZ entirely, so this holds on CI and on a Linux
// or macOS laptop and does nothing here. It is the second line of defence,
// not the first: the clock-pinned fixtures are built with the local-time
// Date constructor so that they hold in any zone on their own.
process.env.TZ ??= "UTC";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
    css: false,
    // The dialog tests type long summaries through user events; under a
    // full parallel run they need more than the 5 s default.
    testTimeout: 15000,
  },
});
