/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { fileURLToPath } from "node:url";

export default defineConfig({
  plugins: [react()],
  // Read @agile-suite/core from its folder, not through the npm workspace
  // link: a Windows policy here refuses to follow node_modules junctions.
  resolve: {
    alias: [
      { find: /^@agile-suite\/core\/(.*)$/, replacement: fileURLToPath(new URL("../../frontend/core/$1", import.meta.url)) },
      { find: "@agile-suite/core", replacement: fileURLToPath(new URL("../../frontend/core/src/index.ts", import.meta.url)) },
    ],
  },
  test: {
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
    css: false,
    testTimeout: 15000,
  },
});
