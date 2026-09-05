import { QueryClient } from "@tanstack/react-query";

// Tuned for a local Go process behind Wails rather than a network: nothing
// retries and nothing refetches in the background. Freshness comes from
// explicit invalidation after mutations.
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: 0,
        refetchOnWindowFocus: false,
        staleTime: 30_000,
      },
    },
  });
}
