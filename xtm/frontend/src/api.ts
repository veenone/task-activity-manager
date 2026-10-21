// api.ts — typed access to the Wails Go backend.
//
// The data interfaces below are plain shapes that match the JSON the backend
// returns. They are intentionally separate from the generated wailsjs model
// classes so plain object literals (initial state, query objects) type-check.
//
// The shapes and bindings live in api/, one module per domain; this file
// only gathers them, so every import of "./api" and every mock of it
// stays one module.

export { EventsOn, BrowserOpenURL } from "../wailsjs/runtime/runtime";
export { errMsg } from "@agile-suite/core";

export * from "./api/bindings";
export * from "./api/app";
export * from "./api/tests";
export * from "./api/changes";
export * from "./api/datasets";
export * from "./api/runs";
export * from "./api/coverage";
export * from "./api/quality";
export * from "./api/imports";
export * from "./api/bugs";
