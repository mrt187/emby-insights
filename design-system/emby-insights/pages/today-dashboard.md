# Today Dashboard Page Overrides

> **PROJECT:** Emby Insights
> **Generated:** 2026-07-29 01:21:05
> **Page Type:** Dashboard / Data View

> ⚠️ **IMPORTANT:** Rules in this file **override** the Master file (`design-system/MASTER.md`).
> Only deviations from the Master are documented here. For all other rules, refer to the Master.

---

## Page-Specific Rules

### Layout Overrides

- **Max Width:** 1200px (standard)
- **Layout:** Full-width sections, centered content
- **Sections:** 1. Hero (product + live preview or status), 2. Key metrics/indicators, 3. How it works, 4. CTA (Start trial / Contact)

### Spacing Overrides

- No overrides — use Master spacing

### Typography Overrides

- No overrides — use Master typography

### Color Overrides

- **Strategy:** Dark, cinematic, and content-first. Keep Emby green as the primary
  accent, cobalt for personal statistics, warm amber for upcoming releases, and
  violet for genre. Never use the rose palette in the generated master file for
  this product.
- **Tokens:** `--ink #090c10`, `--mint-bright #8ce0ad`, `--cobalt #77b8ff`,
  `--ember #ff9e64`, `--violet #c2adff`.

### Component Overrides

- Avoid: Desktop-first causing mobile issues
- Avoid: Large blocking CSS files
- Avoid: Default keyboard for all inputs
- Use a welcome hero with a real personal data point, layered ambient geometry,
  and one clear action. Do not add a generic marketing video or CTA to the app.
- Use a compact bento metric grid and poster-led content rows. Keep the visual
  emphasis on the user’s media, not on decorative chrome.
- The weekly summary is one unified profile card, not four separate metric
  cards: use the real Emby avatar, a bold name, a primary watch-time panel, and
  three compact supporting statistics. Match the charcoal panel, bright text,
  rounded geometry, and emerald accent language of the supplied dashboard
  reference.

---

## Page-Specific Components

- No unique components for this page

---

## Recommendations

- Effects: Expo.out Bezier(0.16,1,0.3,1) easing; spring modals (damping:20 stiffness:90); haptic-linked press (Impact Light/Medium); animated ambient light blobs (Reanimated translateX/Y slow oscillation); BlurView glassmorphism headers/nav (intensity 20); scale press 0.97 → 1.0; avoid pure #000000 (OLED smear)
- Responsive: Start with mobile styles then add breakpoints
- Performance: Inline critical CSS defer non-critical
- Forms: Use inputmode attribute
- CTA Placement: Primary CTA in nav + After metrics
