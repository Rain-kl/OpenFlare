import unusedImports from 'eslint-plugin-unused-imports';
import nextCoreWebVitals from 'eslint-config-next/core-web-vitals';
import nextTypescript from 'eslint-config-next/typescript';

const eslintConfig = [
  ...nextCoreWebVitals,
  ...nextTypescript,
  {
    ignores: [
      'node_modules/**',
      '.next/**',
      'out/**',
      'build/**',
      'next-env.d.ts',
    ],
  },
  {
    plugins: {
      'unused-imports': unusedImports,
    },
    rules: {
      // Prefer unused-imports so unused imports can be auto-removed with --fix.
      '@typescript-eslint/no-unused-vars': 'off',
      'unused-imports/no-unused-imports': 'error',
      'unused-imports/no-unused-vars': [
        'warn',
        {
          vars: 'all',
          varsIgnorePattern: '^_',
          args: 'after-used',
          argsIgnorePattern: '^_',
        },
      ],
      // eslint-config-next 16 启用的 react-hooks v7 新规则对既有代码大量告警，
      // 属 React Compiler 时代的最佳实践；此处先豁免，后续可分批采纳修复。
      'react-hooks/set-state-in-effect': 'off',
      'react-hooks/static-components': 'off',
      'react-hooks/incompatible-library': 'off',
      'react-hooks/preserve-manual-memoization': 'off',
      'react-hooks/refs': 'off',
      'react-hooks/purity': 'off',
      'react-hooks/immutability': 'off',
      '@next/next/no-location-assign-relative-destination': 'off',
    },
  },
];

export default eslintConfig;
