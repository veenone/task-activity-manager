import React from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient } from "@agile-suite/core";
import "@agile-suite/core/styles/tokens.css";
import "@agile-suite/core/styles/primitives.css";
import "./App.css";
import App from "./App";
import { profileBackend } from "./profileBackend";
import { ViewProvider } from "./nav";
import { ModalProvider } from "./modals";

const queryClient = createQueryClient();

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <ViewProvider>
            <ModalProvider>
              <App />
            </ModalProvider>
          </ViewProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>
  </React.StrictMode>,
);
