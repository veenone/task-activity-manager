import React from "react";
import { createRoot } from "react-dom/client";
import { DialogProvider, applyTheme } from "@agile-suite/core";
import "@agile-suite/core/styles/tokens.css";
import "@agile-suite/core/styles/primitives.css";
import "./App.css";
import App from "./App";
import { Theme } from "../wailsjs/go/main/App";

// The stored theme goes on before App renders, because the top bar's toggle
// reads the theme showing when App starts. A failed read follows the OS.
Theme()
  .catch(() => "system")
  .then((theme) => {
    applyTheme(theme);
    createRoot(document.getElementById("root")!).render(
      <React.StrictMode>
        <DialogProvider>
          <App />
        </DialogProvider>
      </React.StrictMode>,
    );
  });
