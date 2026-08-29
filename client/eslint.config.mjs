import eslint from '@eslint/js';
import tseslint from 'typescript-eslint';
import angular from 'angular-eslint';
import prettier from 'eslint-config-prettier/flat';

export default tseslint.config(
    {
        ignores: [
            'dist/**',
            'coverage/**',
            '.angular/**',
            'playwright-report/**',
            'test-results/**',
            'out-tsc/**'
        ]
    },
    {
        files: ['**/*.ts'],
        extends: [
            eslint.configs.recommended,
            ...tseslint.configs.recommendedTypeChecked,
            ...angular.configs.tsRecommended,
            prettier
        ],
        languageOptions: {
            parserOptions: {
                projectService: true,
                tsconfigRootDir: import.meta.dirname
            }
        },
        processor: angular.processInlineTemplates,
        rules: {
            '@angular-eslint/prefer-standalone': 'off',
            '@angular-eslint/component-selector': [
                'error',
                { type: 'element', prefix: ['app', 'ui'], style: 'kebab-case' }
            ],
            '@angular-eslint/directive-selector': [
                'error',
                { type: 'attribute', prefix: ['app', 'ui'], style: 'camelCase' }
            ],
            '@typescript-eslint/no-unused-vars': [
                'error',
                {
                    argsIgnorePattern: '^_',
                    varsIgnorePattern: '^_',
                    caughtErrorsIgnorePattern: '^_'
                }
            ],
            '@typescript-eslint/no-unused-expressions': ['error', { allowTernary: true }],
            '@typescript-eslint/consistent-type-assertions': [
                'error',
                { assertionStyle: 'as', objectLiteralTypeAssertions: 'never' }
            ],
            // ignoreStatic: `Validators.required` & co. are static and never use `this`.
            '@typescript-eslint/unbound-method': ['error', { ignoreStatic: true }],
            '@angular-eslint/prefer-inject': 'error',
            '@angular-eslint/no-output-native': 'error',
            '@angular-eslint/no-output-on-prefix': 'error',
            '@typescript-eslint/no-explicit-any': 'error',
            '@typescript-eslint/no-unsafe-call': 'error',
            '@typescript-eslint/no-unsafe-enum-comparison': 'error',
            '@typescript-eslint/no-base-to-string': 'error',
            '@typescript-eslint/restrict-template-expressions': 'error',
            '@typescript-eslint/no-floating-promises': 'error',
            '@typescript-eslint/no-misused-promises': 'error',

            '@typescript-eslint/class-literal-property-style': ['error', 'fields'],
            '@typescript-eslint/consistent-generic-constructors': ['error', 'constructor'],
            '@typescript-eslint/consistent-indexed-object-style': ['error', 'record'],
            'default-param-last': 'off',
            '@typescript-eslint/default-param-last': 'error',
            '@typescript-eslint/no-confusing-non-null-assertion': 'error',
            '@typescript-eslint/no-duplicate-enum-values': 'error',
            '@typescript-eslint/no-duplicate-type-constituents': 'error',
            '@typescript-eslint/no-empty-object-type': 'error',
            '@typescript-eslint/no-extra-non-null-assertion': 'error',
            'no-implied-eval': 'off',
            '@typescript-eslint/no-implied-eval': 'error',
            '@typescript-eslint/no-mixed-enums': 'error',
            '@typescript-eslint/no-require-imports': 'error',
            'no-useless-constructor': 'off',
            '@typescript-eslint/no-useless-constructor': 'error',
            'no-unused-private-class-members': 'error',

            '@typescript-eslint/parameter-properties': ['error', { prefer: 'class-property' }],
            'dot-notation': 'off',
            '@typescript-eslint/dot-notation': [
                'error',
                { allowIndexSignaturePropertyAccess: true }
            ],
            '@typescript-eslint/array-type': ['error', { default: 'array' }],
            '@typescript-eslint/consistent-type-definitions': ['error', 'interface'],
            '@typescript-eslint/no-inferrable-types': 'off',
            '@typescript-eslint/no-unnecessary-boolean-literal-compare': 'error',
            '@typescript-eslint/prefer-optional-chain': 'error',
            '@typescript-eslint/explicit-member-accessibility': 'error',
            '@typescript-eslint/explicit-function-return-type': [
                'error',
                {
                    allowExpressions: true,
                    allowTypedFunctionExpressions: true,
                    allowHigherOrderFunctions: true
                }
            ],

            // Backlog tier: every rule below is a genuine finding we have not paid down yet.
            // Counts and the paydown plan live in docs/development/code-quality.roadmap.md
            // (findings 17, 18, 19). Promote to 'error' once a count reaches zero —
            // do not delete the entry and do not silence it with eslint-disable.
            '@typescript-eslint/no-unsafe-assignment': 'error',
            '@typescript-eslint/no-unsafe-member-access': 'error',
            '@typescript-eslint/no-unsafe-argument': 'error',
            '@typescript-eslint/no-unsafe-return': 'error'
        }
    },
    {
        files: ['**/*.spec.ts', '**/*testbed.helper.ts', 'e2e/**/*.ts', 'src/testing/**/*.ts'],
        rules: {
            '@angular-eslint/component-selector': 'off',
            '@angular-eslint/directive-selector': 'off',
            '@typescript-eslint/require-await': 'off',
            '@typescript-eslint/no-empty-function': 'off',
            'no-empty': 'off',
            '@typescript-eslint/no-explicit-any': 'off',
            '@typescript-eslint/unbound-method': 'off',
            '@typescript-eslint/no-unsafe-assignment': 'off',
            '@typescript-eslint/no-unsafe-member-access': 'off',
            '@typescript-eslint/no-unsafe-argument': 'off',
            '@typescript-eslint/no-unsafe-call': 'off',
            '@typescript-eslint/no-unsafe-return': 'off',
            '@typescript-eslint/explicit-function-return-type': 'off'
        }
    },
    {
        files: ['**/*.html'],
        extends: [...angular.configs.templateRecommended, prettier],
        rules: {
            '@angular-eslint/template/use-track-by-function': 'error',
            '@angular-eslint/template/no-any': 'error'
        }
    }
);
