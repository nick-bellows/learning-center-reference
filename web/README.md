# web

> Status: **planned.** Next.js (App Router) + TypeScript + Tailwind, `en` + `es`.

Structure lands in Step 2:

```text
app/[locale]/(learner)/   dashboard, course player, certificates
app/[locale]/(admin)/     roster, compliance dashboard
app/verify/[certId]/      public certificate verification
components/ui/            design-system primitives (from tokens)
lib/                      typed API client, auth, i18n
messages/                 en.json, es.json
styles/tokens.css        color / type scale / spacing tokens
tests/                    vitest unit + playwright a11y
```

Every screen ships keyboard-navigable with visible focus. Accessibility is tested in CI, not asserted.
