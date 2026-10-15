import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

const stylesDir = path.resolve(__dirname, '../styles');
const tokensCss = fs.readFileSync(path.join(stylesDir, 'tokens.css'), 'utf8');
const primitivesCss = fs.readFileSync(path.join(stylesDir, 'primitives.css'), 'utf8');

function declarations(css: string, selector: string): Map<string, string> {
  const start = css.indexOf(`${selector} {`);
  const open = css.indexOf('{', start);
  const close = css.indexOf('}', open);
  // Comments go first: one reading "tokens: they were…" would otherwise
  // swallow the declaration after it.
  const block = (start >= 0 && open >= 0 && close >= 0 ? css.slice(open + 1, close) : '').replace(/\/\*[\s\S]*?\*\//g, '');
  return new Map([...block.matchAll(/([\w-]+)\s*:\s*([^;]+);/g)].map((match) => [match[1], match[2].trim()]));
}

describe('core design tokens', () => {
  const light = declarations(tokensCss, ':root');
  const dark = declarations(tokensCss, ':root[data-theme="dark"]');

  it('defines the shared radius, motion, focus and hit-area scales', () => {
    expect(Object.fromEntries([...light].filter(([name]) => /^(--r-|--dur-|--ease-|--focus-|--hit-)/.test(name)))).toEqual({
      '--r-1': '4px',
      '--r-2': '6px',
      '--r-3': '10px',
      '--r-4': '14px',
      '--r-full': '999px',
      '--dur-1': '120ms',
      '--dur-2': '180ms',
      '--dur-3': '240ms',
      '--ease-out': 'cubic-bezier(0.2, 0, 0, 1)',
      '--focus-ring': '2px solid var(--accent)',
      '--focus-offset': '2px',
      '--focus-offset-inset': '-2px',
      '--hit-min': '24px',
      '--hit-desktop': '40px',
      '--hit-touch': '44px',
    });
  });

  it('defines elevations once and changes only their alpha values for dark mode', () => {
    expect(light.get('--e-0')).toBe('none');
    expect(light.get('--modal-shadow')).toBe('var(--e-3)');
    expect([...dark.keys()].filter((name) => name.startsWith('--e-'))).toEqual(['--e-1', '--e-2', '--e-3']);
    expect(dark.get('--e-1')).toContain('rgba(0, 0, 0, 0.12)');
    expect(dark.get('--e-2')).toContain('rgba(0, 0, 0, 0.56)');
    expect(dark.get('--e-3')).toContain('rgba(0, 0, 0, 0.64)');
  });

  // Type and layer scales (#217). A rule sets a size or a stacking level
  // through these, never a literal, so the steps stay few and in order.
  it('defines the type and layer scales', () => {
    expect(Object.fromEntries([...light].filter(([name]) => /^(--fs-|--z-)/.test(name)))).toEqual({
      '--fs-2xs': '10px',
      '--fs-xs': '11px',
      '--fs-sm': '12px',
      '--fs-md': '13px',
      '--fs-lg': '15px',
      '--fs-xl': '18px',
      '--fs-2xl': '26px',
      '--z-sticky': '1',
      '--z-float': '5',
      '--z-popover': '20',
      '--z-menu': '40',
      '--z-modal': '60',
      '--z-modal-popover': '70',
    });
  });

  // The status palette (#214): a colour a rule needs for danger, warn, ok,
  // info or violet is one of these, never a literal, and each has a dark value
  // so it follows the theme.
  it('gives every status palette token a light and a dark value', () => {
    const families = ['danger', 'warn', 'ok', 'info', 'indigo', 'cyan', 'violet'];
    const status = [...light.keys()].filter((name) => families.some((f) => name.startsWith(`--${f}-`)));
    expect(status.length).toBeGreaterThanOrEqual(families.length * 2);
    expect(status.filter((name) => !dark.has(name))).toEqual([]);
  });

  // The pending button's dot is drawn by this rule alone (#255). XTM's span
  // held a glyph, and a background on it painted the glyph's box: a square.
  it('draws the pending button dot as a circle', () => {
    const dot = declarations(primitivesCss, '.btn-pending .pending-dot');
    expect(dot.get('border-radius')).toBe('50%');
    expect(dot.get('width')).toBe(dot.get('height'));
    expect(dot.get('display')).toBe('inline-block');
  });

  it('applies the elevation and hit-area tokens to shared primitives', () => {
    expect(declarations(primitivesCss, '.menu-panel').get('box-shadow')).toBe('var(--e-2)');
    expect(declarations(primitivesCss, '.btn').get('min-height')).toBe('var(--hit-min)');
  });

  // One warning family (#251). --warning-* grew up beside --warn-* and gave
  // five amber text colours with no rule for choosing; --warn-* matches the
  // other status families, so it is the one that stays.
  it('keeps a single warning family', () => {
    expect(light.get('--warn-bg')).toBe('#fef3c7');
    expect(dark.get('--warn-bg')).toBe('#2a2109');
    const strays = stylesheets().flatMap(({ file, css }) =>
      [...css.matchAll(/--(warning\b[\w-]*|warn-accent)\b/g)].map((m) => `${file}: --${m[1]}`),
    );
    expect(strays).toEqual([]);
  });

  it('keeps warning text readable on both warning surfaces', () => {
    for (const theme of [light, dark]) {
      const surface = resolve(theme, '--surface');
      const text = resolve(theme, '--warn-text');
      expect(contrast(text, resolve(theme, '--warn-bg'))).toBeGreaterThanOrEqual(4.5);
      expect(contrast(text, over(resolve(theme, '--warn-soft'), surface))).toBeGreaterThanOrEqual(4.5);
    }
  });
});

// The stylesheets of every workspace: core's styles and each app's src tree.
function stylesheets(): { file: string; css: string }[] {
  const root = path.resolve(__dirname, '../../..');
  const dirs = ['frontend/core/styles', 'frontend/core/src', 'tam/frontend/src', 'xtm/frontend/src'];
  const out: { file: string; css: string }[] = [];
  const walk = (dir: string) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(full);
      else if (entry.name.endsWith('.css')) out.push({ file: path.relative(root, full), css: fs.readFileSync(full, 'utf8') });
    }
  };
  dirs.forEach((d) => walk(path.join(root, d)));
  return out;
}

type Rgba = [number, number, number, number];

// resolve follows var() references in a theme, falling back to the light
// block for names the dark block does not override.
function resolve(theme: Map<string, string>, name: string): Rgba {
  const lightBlock = declarations(tokensCss, ':root');
  let value = theme.get(name) ?? lightBlock.get(name) ?? '';
  for (let i = 0; i < 10; i++) {
    const ref = value.match(/^var\((--[\w-]+)\)$/);
    if (!ref) break;
    value = theme.get(ref[1]) ?? lightBlock.get(ref[1]) ?? '';
  }
  return parseColour(value);
}

function parseColour(value: string): Rgba {
  const rgba = value.match(/^rgba\((\d+),\s*(\d+),\s*(\d+),\s*([\d.]+)\)$/);
  if (rgba) return [Number(rgba[1]), Number(rgba[2]), Number(rgba[3]), Number(rgba[4])];
  let hex = value.replace('#', '');
  if (hex.length === 3) hex = [...hex].map((c) => c + c).join('');
  if (!/^[0-9a-f]{6}$/i.test(hex)) throw new Error(`not a colour: ${value}`);
  return [parseInt(hex.slice(0, 2), 16), parseInt(hex.slice(2, 4), 16), parseInt(hex.slice(4, 6), 16), 1];
}

// over composites a translucent colour onto an opaque one.
function over([r, g, b, a]: Rgba, [br, bg, bb]: Rgba): Rgba {
  return [r * a + br * (1 - a), g * a + bg * (1 - a), b * a + bb * (1 - a), 1];
}

function contrast(fg: Rgba, bg: Rgba): number {
  const lum = ([r, g, b]: Rgba) =>
    [r, g, b]
      .map((c) => c / 255)
      .map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
      .reduce((sum, c, i) => sum + c * [0.2126, 0.7152, 0.0722][i], 0);
  const [hi, lo] = [lum(fg), lum(bg)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}
