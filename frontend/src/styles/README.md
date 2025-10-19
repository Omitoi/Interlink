
# styles/README.md

Interlink – CSS style guide & utilities (2025-09)

This document explains how our CSS is organized and how to use the shared tokens and utilities when building or updating UI.

---

## 0) Philosophy

- **One source of truth** for look & feel: foundational **tokens** (`--color-*`, `--space-*`, `--leading`, `--radius-*`, `--shadow-*`, `--bp-*`).
- **Tiny base layer**, **light utilities**, **scoped modules**. Utilities provide common patterns (card, stack/inline, clamp), modules handle layout specifics only.
- Prefer **tokens & utilities** over ad‑hoc CSS in modules.

---

## 1) Folder structure

```bash
styles/
  foundation/
    tokens.css   # design tokens (colors, spacing, radii, shadows, breakpoints)
    reset.css    # CSS reset
    base.css     # global defaults (links, focus ring, typography, tables, container helper)
  utilities/
    utilities.css # .u-* helpers (card, stack/inline, container queries, line-clamp, buttons, inputs, etc.)
```

> Component/page CSS Modules live next to components/pages (e.g. `src/pages/Profile.module.css`).

---

## 2) Design tokens (tokens.css)

### Colors

Use `--color-*` only.

- `--color-bg`, `--color-surface`, `--color-text`, `--color-text-muted`, `--color-border`
- Brand: `--color-primary`, `--color-primary-contrast`, `--color-success`, `--color-danger`

### Spacing scale (rem-based)

```css
--space-0: 0;
--space-1: 0.25rem;  /* 4px */
--space-2: 0.5rem;   /* 8px */
--space-3: 0.75rem;  /* 12px */
--space-4: 1rem;     /* 16px */
--space-5: 1.5rem;   /* 24px */
--space-6: 2rem;     /* 32px */
--space-7: 3rem;     /* 48px */
--space-8: 4rem;     /* 64px */
```

### Typography

- Fonts: `--font-sans`, `--font-mono`
- Line-height: `--leading` (default body rhythm)

### Radii & shadows

- Radii: `--radius-xs/sm/md/lg/xl`
- Shadows: `--shadow-sm/md/lg`

### Breakpoints

- `--bp-sm: 480px; --bp-md: 768px; --bp-lg: 1024px; --bp-xl: 1280px;`

> **Important:** Native CSS does **not** support `var()` in `@media`. Use **hard-coded px** values in media queries.

---

## 3) Base layer (base.css)

- Links are **underlined by default** (accessible + modern), with `text-underline-offset` and `text-decoration-thickness` for a clean look.
- Global **focus ring**:

  ```css
  :where(a, button, input, select, textarea, [role="button"], [tabindex]:not([tabindex="-1"])):focus-visible {
    outline: 2px solid var(--color-primary);
    outline-offset: 3px;
  }
  ```

- Body uses `--leading`. Basic inputs/tables have sane defaults.
- A tiny `.container` helper exists for page max-width + side padding.

---

## 4) Utilities (utilities.css)

### Card

```css
.u-card { background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius-lg); box-shadow: var(--shadow-sm); padding: var(--space-5); }
.u-card--sm { padding: var(--space-4); }
.u-card--lg { padding: var(--space-6); }
```

Usage in TSX:

```tsx
<div className={`u-card`}>…</div>
<div className={`u-card u-card--lg`}>…</div>
```

### Stack / Inline spacing

```css
.u-stack > * + * { margin-top: var(--space-5); }
.u-stack--sm > * + * { margin-top: var(--space-3); }
.u-stack--lg > * + * { margin-top: var(--space-6); }

.u-inline > * + * { margin-left: var(--space-5); }
.u-inline--sm > * + * { margin-left: var(--space-3); }
.u-inline--lg > * + * { margin-left: var(--space-6); }
```

### Container queries

```css
.u-container { container-type: inline-size; container-name: content; }
.u-grid-2 { display: grid; grid-template-columns: 1fr; gap: var(--space-5); }
@container content (min-width: 700px) {
  .u-grid-2 { grid-template-columns: 1fr 1fr; }
}
```

### Line clamp (multi-line ellipsis)

```css
.u-line-clamp { display: -webkit-box; -webkit-box-orient: vertical; overflow: hidden; }
.u-line-clamp--2 { line-clamp: 2; -webkit-line-clamp: 2; }
.u-line-clamp--3 { line-clamp: 3; -webkit-line-clamp: 3; }
.u-line-clamp--4 { line-clamp: 4; -webkit-line-clamp: 4; }
```

### Button & input baselines (utility-level)

```css
.u-btn { display:inline-flex; align-items:center; justify-content:center; gap:var(--space-3);
  padding:var(--space-3) var(--space-4); border:1px solid var(--color-border); border-radius:var(--radius-md);
  background:var(--color-surface); color:inherit; cursor:pointer; transition: box-shadow 120ms ease, transform 120ms ease, border-color 120ms ease; }
.u-btn--primary { background:var(--color-primary); color:var(--color-primary-contrast); border-color:transparent; }
.u-input { width:100%; padding:var(--space-3) var(--space-3); border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-surface); color:inherit; }
```

---

## 5) Module CSS conventions

- **Use utilities for visuals**; modules define **layout only**.
  - Example: a card module keeps `display`, `gap`, etc., while `.u-card` provides background/border/shadow/padding.
- **No global element selectors** in modules (avoid `button {}` leaks). Always scope: `.actions button { … }`.
- Spacing: `--space-*` tokens or utilities (`.u-stack`, `.u-inline`) — avoid hard px.
- Truncation: use `.u-line-clamp--N` (standard + WebKit).
- Media queries: hard px or `@custom-media` (see §2).

---

## 6) Accessibility & usability

- Keep links underlined in running text. If a link must look like a button, use a real button or `.u-btn`.
- Do **not** remove focus outlines; rely on the base focus ring.
- Aim for ~44px tappable height for primary actions (component-level concern).
- Check contrast in dark mode (badges/labels especially).

---

## 7) Quick recipes

**Make a card with tidy spacing:**

```tsx
<div className="u-card u-stack">
  <h2>Title</h2>
  <p>Content…</p>
  <button className="u-btn u-btn--primary">Action</button>
</div>
```

**Container-aware two-column list (auto 1→2 cols):**

```tsx
<div className="u-container">
  <ul className="u-grid-2">…</ul>
</div>
```

**Multi-line truncate to 3 lines:**

```html
<p class="u-line-clamp u-line-clamp--3">Long text…</p>
```

---

## 8) Testing checklist (per page)

- Links are underlined; buttons look like buttons.  
- Focus ring visible on all interactive elements (Tab).  
- Card visuals consistent across pages; no double padding.  
- Spacing rhythm matches tokens; no stray px gaps.  
- Chat: sidebar stacks under 1024px; scroll works; messages wrap and clamp as expected.  
- Dark mode: text, borders and badges keep good contrast.
