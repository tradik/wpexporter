# Site assets

Images the documentation site ([docs-site.yaml](../docs-site.yaml), built into
`.site/`) resizes at build time. Nothing here is shipped in the binaries or the
Docker image — these files exist for <https://wpexporter.tradik.com/> only.

| File | Used as | Requirements |
|---|---|---|
| `logo.png` | `variables.logo` — brand mark beside the site name, rendered 36 px tall in the header and 48 px in the footer | **Must carry its own alpha.** A mark saved on a white plate shows as a white box in dark mode. Ship it at 2× (roughly 96 px tall) or larger; SSG resizes down. |
| `wpexporter_back.jpg` | `variables.hero_image` — home-page hero photograph, resized to 1600/900 WebP | Landscape, ≥ 1600 px wide. A scrim built from the canvas colour is laid over it, so a busy photo still keeps the heading at AA/AAA contrast. |

The hero is in place; **`logo.png` is still unset**, so its `variables` line in
`docs-site.yaml` stays commented out and the header renders as a wordmark. Drop a
file in here, uncomment its line, and it appears — no template change.

The theme's accent ramp in
[`templates/ssgtheme/css/tokens.css`](../templates/ssgtheme/css/tokens.css) is
taken from this photograph's sky, and the ink ramp is warmed to the same light.
**Replacing the hero with a picture of another colour means retuning those two
ramps**, or the page will sit on the image rather than in it — and every ratio
noted beside a token has to be re-measured, not assumed.

WebP encoding of the hero needs `cwebp` on the machine doing the build
(`apt-get install webp`); the CI workflow installs it. The logo is rendered as PNG
precisely so a machine without `cwebp` still builds the site.
