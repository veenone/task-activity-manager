// Detail panels, filter bars and page margins come from core (#541). XTM's
// test, requirement and precondition details used three header layouts and
// a centre-aligned field grid, its filter bar was a full-width band, and its
// page margin was 0, 16 or 22px depending on the view. jsdom applies no
// stylesheet, so these read the rules and the markup that names them.
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

const repoRoot = path.resolve(__dirname, '../../..');
const read = (rel: string) => fs.readFileSync(path.join(repoRoot, rel), 'utf8').replace(/\/\*[\s\S]*?\*\//g, ' ');

const primitives = read('frontend/core/styles/primitives.css');
const tokens = read('frontend/core/styles/tokens.css');
const tamCss = read('tam/frontend/src/App.css');
const xtmCss = read('xtm/frontend/src/App.css');

/** Declarations of the rules whose selector list names `selector` exactly. */
function rule(css: string, selector: string): string {
  const out: string[] = [];
  for (const chunk of css.split('}')) {
    const at = chunk.lastIndexOf('{');
    if (at < 0) continue;
    const head = chunk.slice(0, at);
    const sel = head.slice(head.lastIndexOf('{') + 1);
    if (sel.split(',').some((part) => part.trim() === selector)) out.push(chunk.slice(at + 1));
  }
  return out.join(' ');
}

const SHARED = ['.detail-head', '.detail-head-id', '.detail-head-actions', '.detail-key', '.detail-fields', '.detail-fields dt', '.detail-fields dd', '.filter-bar'];

describe('shared detail and filter rules', () => {
  it.each(SHARED)('core defines %s', (selector) => {
    expect(rule(primitives, selector)).toMatch(/\S/);
  });

  it.each(SHARED)('neither app redefines %s', (selector) => {
    expect(rule(tamCss, selector)).toBe('');
    expect(rule(xtmCss, selector)).toBe('');
  });

  it('the field grid is top-aligned and lets values wrap', () => {
    expect(rule(primitives, '.detail-fields')).toMatch(/align-items:\s*start/);
    expect(rule(primitives, '.detail-fields dd')).toMatch(/overflow-wrap:\s*anywhere/);
  });
});

describe('page margins', () => {
  it('core owns the page gutter', () => {
    expect(rule(tokens, ':root')).toMatch(/--page-gutter:\s*\d+px/);
  });

  it('TAM pads its page with the gutter', () => {
    expect(rule(tamCss, '.main')).toMatch(/padding:\s*var\(--page-gutter\)/);
  });

  it('every XTM view pads with the gutter, and none sets a margin of its own', () => {
    expect(rule(xtmCss, '.content')).toMatch(/padding:\s*var\(--page-gutter\)/);
    for (const view of ['dashboard', 'preconditions', 'containers', 'requirements', 'coverage', 'components']) {
      expect(rule(xtmCss, `.content-${view}`)).not.toMatch(/(^|[;\s])padding\s*:/);
    }
  });
});

describe('XTM detail markup', () => {
  const tsx = (rel: string) => read(`xtm/frontend/src/components/${rel}`);

  it('the requirement and precondition details use the shared header and grid', () => {
    const req = tsx('RequirementsView.tsx');
    const pre = tsx('preconditions/DetailHeader.tsx');
    expect(req).toMatch(/className="detail-head"/);
    expect(req).toMatch(/className="detail-fields"/);
    expect(pre).toMatch(/className="detail-head"/);
    expect(`${req}${pre}`).not.toMatch(/reqs-detail-head|precond-detail-head|detail-meta/);
  });

  it('the test table filter bar is the shared one', () => {
    expect(tsx('test-table/FilterBar.tsx')).toMatch(/className="filter-bar"/);
  });
});
