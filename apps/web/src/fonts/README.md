# Self-hosted fonts

Variable WOFF2 files, latin subset, served by `next/font/local` from
`src/app/layout.tsx` so a build never depends on fonts.googleapis.com.

| File | Family | Axes | Source |
|---|---|---|---|
| `fraunces-latin.woff2`, `fraunces-italic-latin.woff2` | Fraunces | wght 100–900, opsz 9–144, SOFT 0–100 | Google Fonts build of github.com/undercasetype/Fraunces |
| `inter-tight-latin.woff2` | Inter Tight | wght 100–900 | Google Fonts build of github.com/rsms/inter |
| `jetbrains-mono-latin.woff2` | JetBrains Mono | wght 100–800 | Google Fonts build of github.com/JetBrains/JetBrainsMono |

All three are licensed under the SIL Open Font License 1.1
(https://openfontlicense.org), which permits bundling and self-hosting.
Fetched 2026-09-22 from the Google Fonts CSS API (latin `unicode-range`
block). To refresh, request the same families from
`https://fonts.googleapis.com/css2` with a WOFF2-capable user agent and
replace the files; keep the file names, the layout references them.
