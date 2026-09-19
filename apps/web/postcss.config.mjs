// Tailwind v4 goes through this single PostCSS plugin; no tailwind.config.js
// is required in v4 — design tokens and content globs live in globals.css.
const config = {
  plugins: {
    "@tailwindcss/postcss": {},
  },
};

export default config;
