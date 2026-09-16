import { useEffect, useId, useRef, useState } from "react";

interface Props {
  href: string | null;
  onApply: (href: string) => string | null;
  onRemove: () => void;
  // onClose is called on Apply, Remove and Escape; the toolbar returns focus
  // to the link button from it.
  onClose: () => void;
}

// LinkPopover is the address box under a toolbar's link button. A refusal
// from onApply stays on screen under the box, rather than the box closing on
// an address that was never applied.
export function LinkPopover({ href, onApply, onRemove, onClose }: Props) {
  const [value, setValue] = useState(href ?? "");
  const [error, setError] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const errorId = useId();

  useEffect(() => {
    input.current?.focus();
    input.current?.select();
  }, []);

  const apply = () => {
    const refusal = onApply(value);
    if (refusal) {
      setError(refusal);
      return;
    }
    onClose();
  };

  return (
    <div
      className="editor-link-popover"
      role="dialog"
      aria-label="Link"
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          e.preventDefault();
          e.stopPropagation();
          onClose();
        }
      }}
    >
      <input
        ref={input}
        aria-label="Link address"
        value={value}
        placeholder="https://"
        spellCheck={false}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : undefined}
        onChange={(e) => {
          setValue(e.target.value);
          setError("");
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            apply();
          }
        }}
      />
      <button type="button" className="btn btn-primary" onClick={apply}>Apply</button>
      {href !== null && (
        <button
          type="button"
          className="btn btn-ghost"
          onClick={() => {
            onRemove();
            onClose();
          }}
        >
          Remove
        </button>
      )}
      {error && <p id={errorId} className="error-text editor-link-error" role="alert">{error}</p>}
    </div>
  );
}
