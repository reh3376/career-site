// ESLint flat config. eslint-config-next 16 exports true flat-config
// arrays for each preset, so the FlatCompat / @eslint/eslintrc bridge
// we needed on eslint-config-next 15 is gone. Importing the arrays
// directly keeps us off the buggy @eslint/eslintrc validator that was
// crashing on 15→16 attempts.
import nextCoreWebVitals from "eslint-config-next/core-web-vitals";
import nextTypescript from "eslint-config-next/typescript";

const config = [
  { ignores: [".next/**", "node_modules/**", "src/gen/**", "next-env.d.ts"] },
  ...nextCoreWebVitals,
  ...nextTypescript,
];

export default config;
