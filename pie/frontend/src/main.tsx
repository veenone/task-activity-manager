import React from "react";
import { createRoot } from "react-dom/client";
import { DialogProvider, applyTheme } from "@agile-suite/core";
import "@agile-suite/core/styles/tokens.css";
import "@agile-suite/core/styles/primitives.css";
import "./App.css";
import App from "./App";
import { Theme } from "../wailsjs/go/main/App";

// The stored theme, before the first paint where possible. A failed read keeps
// the OS's choice.
applyTheme("system");
Theme().then(applyTheme, () => undefined);

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <DialogProvider>
      <App />
    </DialogProvider>
  </React.StrictMode>,
);
