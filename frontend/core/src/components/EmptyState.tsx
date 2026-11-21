import type { ReactNode } from "react";

// filteredEmpty is the one sentence every list says when its own filter, not
// the data, is why nothing shows. noun is what the list holds, plural.
export function filteredEmpty(noun: string): string {
  return `No ${noun} match the current filter.`;
}

interface EmptyStateProps {
  // filtered says a filter is set. The list's own empty line (sync, create,
  // add) would send the reader the wrong way then, so the filter is named.
  filtered: boolean;
  noun: string;
  className?: string;
  children: ReactNode;
}

// EmptyState is the line a list shows when it has nothing to draw (#532).
// Never render it next to a LoadError: a failed load says nothing about
// whether the data is empty.
export function EmptyState({ filtered, noun, className, children }: EmptyStateProps) {
  return <p className={className ? `muted ${className}` : "muted"}>{filtered ? filteredEmpty(noun) : children}</p>;
}

interface LoadErrorProps {
  message: string;
  onRetry: () => void;
  className?: string;
}

// LoadError is a failed fetch with a way to try again. It stands in for the
// data, never for the controls that chose it, so a filter that caused the
// error can still be changed.
export function LoadError({ message, onRetry, className }: LoadErrorProps) {
  return (
    <p className={className ? `error-text ${className}` : "error-text"} role="alert">
      {message}{" "}
      <button type="button" className="btn" onClick={onRetry}>Retry</button>
    </p>
  );
}
