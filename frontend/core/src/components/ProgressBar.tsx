
// BarTone is what the bar is measuring, which is the only thing that varies
// between one bar and the next. It is not a colour: which fill each tone
// draws is one token in tokens.css.
export type BarTone = "time" | "points" | "muted";

interface Props {
  value: number;
  max: number;
  // The accessible name, e.g. "Time elapsed in Sprint 14". A bar takes no
  // name from the text beside it, so without this it is an unnamed
  // progressbar and a reader hears a percentage with nothing attached.
  label: string;
  // What the value means in words, e.g. "Day 6 of 14". The number alone is
  // read as a bare percentage.
  valueText: string;
  // Where to draw a reference line, 0 to 1 along the track. A fraction
  // outside that is not on the track, so it is dropped rather than drawn
  // past either end.
  marker?: number;
  tone?: BarTone;
}

// ProgressBar is one track, one fill, and an optional marker. It takes
// numbers and knows nothing about time, sprints or cards: what the numbers
// mean is entirely in the two strings the caller passes.
export function ProgressBar({ value, max, label, valueText, marker, tone = "time" }: Props) {
  // A scope with nothing in it has nothing to divide by. Answering 0 rather
  // than NaN matters twice: a width of NaN% is a declaration the browser
  // drops, which leaves the fill at whatever it inherited, and aria-valuenow
  // of NaN is not a number a reader can be told.
  const span = Number.isFinite(max) && max > 0 ? max : 0;
  const now = span > 0 && Number.isFinite(value) ? Math.min(Math.max(value, 0), span) : 0;
  const filled = span > 0 ? (now / span) * 100 : 0;
  const at = marker !== undefined && marker >= 0 && marker <= 1 ? marker : undefined;
  return (
    <div
      className={`progress-bar progress-bar-${tone}`}
      role="progressbar"
      aria-label={label}
      aria-valuenow={now}
      aria-valuemin={0}
      aria-valuemax={span}
      aria-valuetext={valueText}
    >
      <span className="progress-bar-fill" style={{ width: `${filled}%` }} />
      {at !== undefined && (
        <span className="progress-bar-marker" aria-hidden="true" style={{ left: `${at * 100}%` }} />
      )}
    </div>
  );
}
