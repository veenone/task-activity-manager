import React from "react";
import { createRoot } from "react-dom/client";
import { DialogProvider, applyTheme } from "@agile-suite/core";
import "@agile-suite/core/styles/tokens.css";
import "@agile-suite/core/styles/primitives.css";
import "./App.css";
import App from "./App";

applyTheme("system");

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <DialogProvider>
      <App />
    </DialogProvider>
  </React.StrictMode>,
);
