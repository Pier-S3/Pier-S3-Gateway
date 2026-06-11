# Brand assets

Logo variants for Pier S3 Gateway. All are SVG (crisp at any size).

| File | Use |
|------|-----|
| [`logo-icon.svg`](logo-icon.svg) | App icon - rounded blue tile with the white pier/parcel mark. Use as the square avatar/social icon. |
| [`logo-icon-mono.svg`](logo-icon-mono.svg) | Single-color outline icon (`currentColor`). For monochrome / print / stamps; set color via CSS or `fill`/`stroke`. |
| [`logo-horizontal.svg`](logo-horizontal.svg) | Horizontal lockup (icon + "Pier S3 / GATEWAY") for **light** backgrounds. |
| [`logo-horizontal-dark.svg`](logo-horizontal-dark.svg) | Horizontal lockup for **dark** backgrounds (light wordmark). |

The in-app mark lives separately in `web/src/components/Logo.tsx` (uses
`currentColor` so it follows the active theme) and `web/public/favicon.svg`.

## UI screenshots

Captures of the running web UI live in [`screenshots/`](screenshots/) and are
referenced from the project [README](../../README.md#screenshots).

| File | Shows |
|------|-------|
| [`screenshots/buckets-light.png`](screenshots/buckets-light.png) | Bucket grid with per-bucket RO/RW/WO access badges (default theme). The README hero. |
| [`screenshots/buckets-dark.png`](screenshots/buckets-dark.png) | Same view in the Dark theme. |
| [`screenshots/buckets-claude.png`](screenshots/buckets-claude.png) | Same view in the Claude (warm ivory/clay) theme. |
| [`screenshots/object-browser.png`](screenshots/object-browser.png) | Object browser - file table with name/modified/size and folder navigation. |
| [`screenshots/file-preview.png`](screenshots/file-preview.png) | In-app file preview modal with download. |
| [`screenshots/login.png`](screenshots/login.png) | OIDC sign-in screen. |

To refresh them, run the dev stack (`make dev-up`), sign in, and re-capture; keep
the filenames stable so the README and docs links keep resolving.

## The mark

A monoline parcel/container (an object in storage) on a pier deck with pilings
above a wave - "Pier" (the dock buckets tie up to) + the object it stores.

## Colors

- Brand blue: `#0061FF`
- Ink (light bg wordmark): `#1e1919` / muted `#637282`
- On dark bg wordmark: `#f5f5f5` / muted `#9aa7b4`

## Exporting to PNG

```bash
# requires librsvg (brew install librsvg) or Inkscape
rsvg-convert -w 512 -h 512 logo-icon.svg -o logo-icon-512.png
```
