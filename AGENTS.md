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

### Product Goal and Skill Responsibilities
HomeWaf is an operational console for protecting home services and websites. Optimize for **complete capabilities, complete user flows, and clear information hierarchy**, then visual consistency. Fewer controls or prettier screenshots are not evidence of a better product.

Read both `.agents/skills/ui-ux-pro-max/SKILL.md` and `.agents/skills/design-taste-frontend/SKILL.md` before frontend design, review, or refactoring. Apply them in this order:

| Skill | Responsibility in this repository | Expected result |
| --- | --- | --- |
| `ui-ux-pro-max` | **PM + UX:** user goals, domain objects, information hierarchy, navigation, prerequisites, interaction states, feedback, recovery, and accessibility. | A concrete task flow and acceptance criteria before changing page structure. |
| `design-taste-frontend` | **Visual design + frontend craft:** implement that flow with existing tokens, layout, typography, density, controls, responsive behavior, and visual QA. | A coherent, usable console that preserves the agreed task flow. |

These are project-specific responsibilities, not claims that either upstream skill covers only one discipline. Their overlap is resolved by task completion and this repository's product constraints. Explicit user decisions and repository rules take precedence over skill defaults and generated recommendations. Use focused `ux` searches and `vue` implementation guidance when useful; assess relevance before applying results. Do not regenerate a palette, font system, navigation scheme, or component stack merely because a skill recommends one. If a local skill is unavailable, report it and use these repository rules without pretending to have read it.

This is an application, not a marketing site. Marketing heroes, full-screen sections, photograph quotas, random design deals, decorative scenes, mandatory entrance animations, and promotional copy/gates do not apply to console pages. Keep useful explanations, chart labels, dense tables, and natural page scrolling. Use relevant accessibility and layout checks plus product-flow tests; never distort a working interface to satisfy an unrelated skill check, or call an open/failed gate a pass.

### Information Hierarchy
- Organize around administrator tasks and real objects: services, requests/events, certificates, access policies, shared resources, and instance administration. Do not turn backend modules or every feature flag into peer navigation items.
- Each page must answer: **what object/scope am I viewing, what is its actual state, what can I do next?** Give the current task a clear primary action when one is needed; keep secondary and destructive actions subordinate. A read-only page does not need an invented CTA.
- Put identity, effective state, and task-critical controls first; show supporting details next and uncommon technical options on demand. Advanced disclosure must not hide required steps, errors, active restrictions, or retained capabilities users need to manage directly. Ordinary tasks must not require editing JSON.
- Choose a list, detail page, tab, inline form, drawer, or dialog by task complexity and context. Avoid duplicate summaries, nested cards, competing toolbars, tab-within-tab navigation, and repeated configuration forms. A domain-specific page must expose its actual task, not just a button opening the entire service editor.
- Give each capability one canonical owner and persistence path. Contextual shortcuts may reuse it while carrying the selected service, certificate, request, filters, and return location. Shared editing logic does not require one giant shared UI. Generate navigation/search from permitted routes; retain useful deep links and predictable Back behavior.
- Keep service configuration, shared resources, and instance settings distinct. In particular, service visitors/accounts, console administrators, and management API keys have different scopes; certificate issuance, certificate storage/binding, and external expiry checks are related but different tasks.

### Complete User Flows
- Specify the entry point, prerequisite, input, validation, commit action, observable result, next step, and cancel/back/recovery path. Cross-page prerequisites should return users to their original task with safe draft state intact.
- Distinguish first use, loading, no records, no filter matches, unavailable/forbidden capability, stale data, and request failure. Show actionable recovery; never turn failed reads into a successful empty or zero state.
- Keep create, edit, save, enable, apply, and verify semantics explicit. Saved configuration is not proof of a running listener; a listener is not proof of reachable traffic; observation is not blocking; a certificate being issued is not proof that a service serves it.
- Preserve input through validation and request failures, tab changes, and safe prerequisite detours. Handle duplicate submits, async races, version conflicts, polling, and unsaved changes. Keep secrets out of persistent browser storage. Explain which actions save the whole draft and which take effect immediately; cancel must not imply undoing an already committed operation.
- Verify the write reaches the intended service, persistence record, and runtime consumer. A new service selector must not feed native service identities into an incompatible legacy write API. Missing backend integration is a functional gap, not something a success toast or visual redesign can solve.
- Keep dangerous actions scoped and explicit about the affected object and consequences. Do not remove authentication, validation, confirmation, or recovery behavior to shorten a flow.

### Refactoring Workflow and Preservation
The whole-console inventory, historical decisions, priorities, and route checklist live in [`docs/modernization/CONSOLE-REFACTOR-PLAN.md`](docs/modernization/CONSOLE-REFACTOR-PLAN.md). Use it for the full refactor and update it as implementation progresses. Earlier redesign/QA documents are historical evidence, not proof that current workflows are complete.

1. **Read before redesigning.** Trace the current route, component, API client, server-side behavior, dependent controls, and relevant tests. Compare affected paths with `52078dfc164a3cb60637f903ba9b630dc64d5f45` and subsequent commits. This commit is a comparison anchor, not a blanket rollback target. The user identifies the intervening non-bug changes, including removals and consolidations, as user-directed adjustments; their current presentation is not a final approved design. Classify what was retired, moved, retained, or left incomplete. Do not restore removed features just because the old code had them, or preserve a confusing layout just because it is newer.
2. **Capture the contract.** For each task, record its user, object/scope, entry and exit, ordered information, main/secondary actions, state/recovery behavior, and evidence-backed behavior to preserve. Map existing capabilities to their proposed destinations, including advanced and hidden entry points. Separate confirmed gaps from hypotheses requiring runtime validation.
   **Reverse-check the backend.** Use the registered route inventory and response fields to trace API → client → reachable control → persistence/runtime. Include tabs, dialogs, drawers, detail views, diagnostics, exports and recovery actions. Classify absent references as missing UI, buried UI, dynamic calls, replaced/retired capability, or backend integration gaps; an unused endpoint is not an instruction to resurrect a feature. A referenced endpoint can still have important response fields missing from its UI. Run `node scripts/audit-backend-apis.mjs` and `node scripts/audit-ui-capabilities.mjs` for reproducible clues, then verify the actual journey.
3. **Refactor by complete journey.** Cover the related pages, dialogs, editors, and backend dependencies together. All pages are in scope for the planned refactor, but a global CSS sweep or mass replacement is not completion. Reuse working behavior and user-tuned decisions; change problematic structure with a stated reason. Ask only about material product tradeoffs that code/history cannot resolve.
4. **Replace and verify.** Remove an obsolete implementation only after its retained capabilities have a verified replacement, or its retirement is supported by the user's scope and change history, and its imports, routes, callers, and migration dependencies have been checked. Keep necessary redirects/migration access, not parallel old/new editing modes. Do not delete an unregistered component merely because it has no menu item.
5. **Record evidence.** Update each affected checklist entry with its destination, preserved capabilities, tests performed, and remaining gaps. Claim completion only when the real task can be completed and revisited, including failure recovery.
   Track route coverage, inherited styling, structural redesign, and interaction verification separately. Opening a parent route does not verify its secondary screens. Use [the secondary-screen checklist](docs/modernization/CONSOLE-SUBPAGE-CHECKLIST.md) and [the backend/UI audit](docs/modernization/BACKEND-UI-CAPABILITY-AUDIT.md) alongside the route checklist.

Build one coherent console using `web/src/ui/`, the current shield branding, local fonts, shared tokens, equal content edges, restrained accents, visible labels, keyboard focus, responsive controls, and reduced-motion support. Preserve useful user preferences such as theme and selected log columns. Extend existing Vue/Element Plus controls before adding an abstraction; repair shared controls at their source instead of layering page-specific overrides or another compatibility framework.

## Testing Guidelines
Write only important tests: authentication/permissions, secrets, configuration persistence, destructive actions, critical navigation, and regressions that could interrupt protection. Use Go `*_test.go`/`TestXxx`, Node tests, and targeted Playwright flows. Avoid cosmetic snapshots, trivial implementation tests, and arbitrary coverage quotas. Inspect affected pages at desktop, tablet, and phone widths, including dark mode and error/empty states.

For frontend changes, run the applicable type checks, tests, build, and focused browser flows. Check the full journey: enter, configure, submit, observe the result, leave, reopen, and edit again; also test failure/retry, cancellation, relevant permissions, and context-preserving navigation. For protection changes, verify persisted configuration and the actual runtime effect with isolated backend tests. Mocked browser responses and successful route mounting prove only their covered frontend behavior, not end-to-end protection or external certificate/notification delivery. Report these limits explicitly. Documentation-only changes require content, references, and checklist-coverage checks, not unrelated application builds.

## Commits, Reviews & Safety
Follow recent `feat:`, `fix:`, and `test:` commits; optional scopes include `feat(ui):`. Describe changes, relevant issues, verification results, and UI screenshots in PRs. Keep commits focused. Use isolated `.cache/dev/runtime/` data; never commit credentials, signing keys, databases, or generated artifacts. Follow `CONTRIBUTING.md` and preserve third-party notices.
