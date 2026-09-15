// ESLint flat config for the three frontend workspaces.
//
// It exists to give AGENTS.md rules real gates: I3 (accessibility is never
// traded away) through jsx-a11y, and C2 (files under 400 lines) through
// max-lines, which the ratchet counts from this linter's JSON rather than a
// hand-rolled shell counter.
//
// Generated bindings and build output are not ours to lint.
import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import reactHooks from 'eslint-plugin-react-hooks';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import globals from 'globals';

export default tseslint.config(
  {
    ignores: [
      '**/node_modules/**',
      '**/dist/**',
      '**/build/**',
      '**/wailsjs/**', // generated; the bindings contract owns these
      '**/*.config.js',
      '**/*.config.ts',
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      globals: { ...globals.browser, ...globals.node },
    },
    plugins: {
      'react-hooks': reactHooks,
      'jsx-a11y': jsxA11y,
    },
    rules: {
      // I3: accessibility is never traded for simplicity or speed.
      ...jsxA11y.configs.recommended.rules,

      // C2: keep files under 400 lines of code. The ratchet holds the count.
      'max-lines': ['warn', { max: 400, skipBlankLines: true, skipComments: true }],

      // Frontend logging contract: no console in a workspace src tree.
      'no-console': 'warn',

      // Correctness, the other half of I3.
      'react-hooks/rules-of-hooks': 'error',
      'react-hooks/exhaustive-deps': 'warn',

      // Unused code is dead code (C2). Underscore-prefixed args are deliberate.
      '@typescript-eslint/no-unused-vars': [
        'warn',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
    },
  },
  {
    // Tests may reach for what production code may not.
    files: ['**/*.test.{ts,tsx}', '**/test/**'],
    rules: {
      'no-console': 'off',
      'max-lines': 'off',
    },
  },
);
