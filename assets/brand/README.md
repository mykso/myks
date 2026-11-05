# myks brand assets

The myks logo is a **vortex**: three staggered arcs (coral, amber, violet)
spinning into one center. The arcs stand for upstream sources (Helm charts, Git
repos, plain YAML) mixed into one rendered output.

## Files

| Path                                                                  | Use                                                                     |
| --------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `logo/myks-mark-on-dark.svg`                                          | Full-color mark for dark backgrounds                                    |
| `logo/myks-mark-on-light.svg`                                         | Full-color mark for white/light backgrounds (darker tints)              |
| `logo/myks-mark-adaptive.svg`                                         | Switches between the two with `prefers-color-scheme` (works in `<img>`) |
| `logo/myks-mark-mono.svg`                                             | Single color, uses `currentColor`; inline it and set `color`            |
| `logo/*-1024.png`                                                     | Raster fallbacks (transparent)                                          |
| `logo/myks-logo-on-dark.svg`, `logo/myks-logo-on-light.svg`           | Horizontal lockup, mark and wordmark, for dark and light backgrounds    |
| `logo/myks-logo-mono.svg`                                             | Horizontal lockup in `currentColor`                                     |
| `logo/myks-logo-stacked-{on-dark,on-light,mono}.svg`                  | Stacked lockup: mark above the wordmark                                 |
| `icon/myks-icon.svg`                                                  | App icon: mark on a plum rounded square                                 |
| `icon/favicon.svg`                                                    | Favicon, simplified to two thicker arcs for 16 px                       |
| `icon/favicon.ico`                                                    | 16 (simplified), 32 and 48 px                                           |
| `icon/apple-touch-icon.png`, `icon/icon-192.png`, `icon/icon-512.png` | Touch and web-manifest icons                                            |
| `social/myks-social.svg`, `social/myks-social.png`                    | Social preview card, 1280×640: lockup over the docs tagline, on plum    |
| `tokens/palette.json`                                                 | Palette as design tokens (W3C DTCG format), source of truth             |
| `tokens/palette.css`                                                  | The same palette as CSS custom properties (`--myks-*`)                  |

## Palette

| Token             | Hex       | Role                               |
| ----------------- | --------- | ---------------------------------- |
| `plum`            | `#1C1030` | Primary dark ground; text on light |
| `deep`            | `#120A1F` | Deepest ground (dark-mode page)    |
| `coral`           | `#FF5A4E` | Primary accent, outer arc          |
| `amber`           | `#FFB020` | Secondary accent, middle arc       |
| `violet`          | `#A27BFF` | Tertiary accent, inner arc         |
| `coral-on-light`  | `#E8443A` | Coral on white (3.9:1)             |
| `amber-on-light`  | `#C77700` | Amber on white (3.5:1)             |
| `violet-on-light` | `#7B3FF2` | Violet on white (5.5:1)            |

## Wordmark

`myks`, always lowercase, set in
[Outfit](https://fonts.google.com/specimen/Outfit) SemiBold (600),
letter-spacing `-0.035em`. In a horizontal lockup the mark is as tall as the
font size, with a gap of about `0.16em` between them.

The lockup SVGs carry the wordmark as outlines, so they render the same without
the font. In the horizontal lockup the wordmark's ink is centered vertically on
the mark's ink; in the stacked one the mark is 1.5× the font size, centered over
the wordmark.

`scripts/build-lockups.py` generates every `logo/myks-logo-*.svg` and
`social/myks-social.svg` from a pinned copy of Outfit, shaped with HarfBuzz.
Rebuild them, then render the card's PNG, with:

```sh
uv run assets/brand/scripts/build-lockups.py
nix shell nixpkgs#resvg -c resvg assets/brand/social/myks-social.svg assets/brand/social/myks-social.png
```

The PNG is the repository's social preview. GitHub has no API for it: upload it
under Settings → General → Social preview.

## Usage rules

- Keep clear space around the mark of at least a quarter of its width.
- Use the full mark from 24 px up. Below that, use `favicon.svg` /
  `favicon.ico`.
- Keep the arc order and colors: coral outside, amber middle, violet inside.
  Don't rotate, mirror, stretch or add effects.
- On colored backgrounds (for example coral), use the mono mark in white.

## Snippets

GitHub README, switching with the viewer's theme:

```html
<picture>
  <source
    media="(prefers-color-scheme: dark)"
    srcset="assets/brand/logo/myks-mark-on-dark.svg"
  />
  <img src="assets/brand/logo/myks-mark-on-light.svg" alt="myks" width="96" />
</picture>
```

Starlight (`astro.config.mjs`), with the files copied into the docs'
`src/assets/` and `public/`:

```js
starlight({
  title: 'myks',
  logo: {
    light: './src/assets/myks-mark-on-light.svg',
    dark: './src/assets/myks-mark-on-dark.svg',
  },
  favicon: '/favicon.svg',
});
```

The palette in CSS:

```css
@import './assets/brand/tokens/palette.css';
.button {
  background: var(--myks-coral);
  color: var(--myks-plum);
}
```
