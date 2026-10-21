// api.ts is the frontend's typed access to the Go backend. It re-exports the
// generated bindings and defines plain shapes for what they return, so state
// and test fixtures can be object literals.
//
// The bindings are re-exported through those shapes rather than raw: the
// generated wailsjs models are classes carrying every column of the shared
// Go struct, so a raw re-export would force fixtures to spell out fields TAM
// never reads. Arguments that are Go structs go through the generated
// class's createFrom so the binding receives the shape it declares.
//
// The shapes and bindings live in api/, one module per domain; this file
// only gathers them, so every import of "./api" and every mock of it
// stays one module.

export { EventsOn, BrowserOpenURL } from "../wailsjs/runtime/runtime";
export type { SyncProgress } from "@agile-suite/core";

export * from "./api/core";
export * from "./api/issues";
export * from "./api/boards";
export * from "./api/reports";
export * from "./api/edits";
export * from "./api/pending";
export * from "./api/importer";
export * from "./api/rituals";
export * from "./api/dashboards";
