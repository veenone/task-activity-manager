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
  const block = start >= 0 && open >= 0 && close >= 0 ? css.slice(open + 1, close) : '';
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

  it('applies the elevation and hit-area tokens to shared primitives', () => {
    expect(declarations(primitivesCss, '.menu-panel').get('box-shadow')).toBe('var(--e-2)');
    expect(declarations(primitivesCss, '.btn').get('min-height')).toBe('var(--hit-min)');
  });
});
