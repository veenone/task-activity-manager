/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

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
