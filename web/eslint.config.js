import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  globalIgnores(['dist', 'coverage']),

  // ── Source files (type-aware strict rules) ──────────────────────────────
  {
    files: ['src/**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.strictTypeChecked,   // type-aware: catches unsafe patterns
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      globals: globals.browser,
      parserOptions: {
        projectService: true,               // enables type-aware analysis
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      // Catch unhandled Promise rejections in async code
      '@typescript-eslint/no-floating-promises': 'error',
      // Prevent async functions passed where void-returning callback is expected,
      // but allow async functions in JSX event attributes (common React pattern)
      '@typescript-eslint/no-misused-promises': [
        'error',
        { checksVoidReturn: { attributes: false } },
      ],
      // Numbers are valid in template literals — `${status}` is intentional
      '@typescript-eslint/restrict-template-expressions': ['error', { allowNumber: true }],
      // Shorthand arrow functions returning void are idiomatic in React JSX handlers
      '@typescript-eslint/no-confusing-void-expression': ['error', { ignoreArrowShorthand: true }],
    },
  },

  // ── Test files (relax unsafe rules — needed for vi.fn() mock patterns) ──
  {
    files: ['src/**/*.test.{ts,tsx}'],
    rules: {
      '@typescript-eslint/no-unsafe-assignment': 'off',
      '@typescript-eslint/no-unsafe-call': 'off',
      '@typescript-eslint/no-unsafe-member-access': 'off',
      '@typescript-eslint/no-unsafe-return': 'off',
      '@typescript-eslint/no-unsafe-argument': 'off',
      '@typescript-eslint/no-explicit-any': 'off',
      // act(async () => {...}) without await is a common testing pattern
      '@typescript-eslint/require-await': 'off',
    },
  },

  // ── Config files + E2E (no project-tsconfig; use basic rules only) ──────
  {
    files: ['*.{js,ts,mjs}', 'e2e/**/*.{ts}'],
    extends: [js.configs.recommended, tseslint.configs.recommended],
    languageOptions: {
      globals: { ...globals.node, ...globals.browser },
    },
  },
])
