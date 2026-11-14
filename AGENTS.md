# Repository Guidelines

## Project Structure & Architecture
HomeWaf protects websites and home services. `cmd/homewaf/` starts the supervisor and workers; `wafmangeweb/` exposes the management server. Follow `router/` → `middleware/` → `api/` → `service/waf_service/` → `model/` and `wafdb/`. `wafpipeline/` owns per-port protection; `wafenginecore/` supplies proxy/runtime integrations. Keep authorization and configuration validation server-side.

`web/` contains Vue 3, TypeScript, Vite, Element Plus, and Pinia. Routes live in `web/src/router/`, views in `pages/`, request clients in `apis/`, shared controls/tokens in `ui/`, and navigation in `layouts/`. Use `utils/request.ts` for authenticated/encrypted requests. Static assets live in `web/public/`; builds embed them into `public/dist/`. Documentation is under `docs/`.

## Build, Test & Development
Use the versions in `go.mod` and `web/package.json` (currently Go 1.27.3 and Node 26.13.0). Run from the repository root:

- `node scripts/dev.mjs`: start both services with reload; Windows also supports `.\dev.bat`. Console: port 3001; backend: 26666.
- `node scripts/build.mjs`: install locked dependencies, type-check, and build frontend/backend into `artifacts/`.
- `node scripts/verify.mjs`: compile every Go package and execute focused regressions.
- `npm run typecheck --prefix web`, `npm test --prefix web`, `npm run build --prefix web`: frontend checks.
- `go test ./wafpipeline -run TestName -count=1`: target a changed behavior.

## Coding & UI Standards
Format Go with `gofmt`; use lowercase packages and descriptive exported names. Frontend formatting follows `web/.editorconfig` and `.prettierrc.cjs`: two spaces, single quotes, semicolons, LF. Use PascalCase Vue components and camelCase functions.

Read `.agents/skills/design-taste-frontend/SKILL.md` before UI work. Build one coherent console: shared tokens, equal content edges, restrained accent, clear headings, visible labels, keyboard focus, responsive controls, and reduced-motion support. Prioritize real service data and task completion. Marketing heroes, decorative effects, and forced full-screen sections do not belong in operational forms. Replace obsolete UI directly; do not add compatibility modes or duplicate implementations.

## Testing Guidelines
Write only important tests: authentication/permissions, secrets, configuration persistence, destructive actions, critical navigation, and regressions that could interrupt protection. Use Go `*_test.go`/`TestXxx`, Node tests, and targeted Playwright flows. Avoid cosmetic snapshots, trivial implementation tests, and arbitrary coverage quotas. Inspect affected pages at desktop, tablet, and phone widths, including dark mode and error/empty states.

## Commits, Reviews & Safety
Follow recent `feat:`, `fix:`, and `test:` commits; optional scopes include `feat(ui):`. Describe changes, relevant issues, verification results, and UI screenshots in PRs. Keep commits focused. Use isolated `.cache/dev/runtime/` data; never commit credentials, signing keys, databases, or generated artifacts. Follow `CONTRIBUTING.md` and preserve third-party notices.
