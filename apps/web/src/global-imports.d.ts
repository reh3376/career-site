// Ambient module declarations for side-effect imports that TypeScript 6.0
// would otherwise reject with TS2882. Scoped to the exact extensions the
// app actually side-effect-imports (CSS today; add others here only when
// we start importing them for side effects).

declare module "*.css";
