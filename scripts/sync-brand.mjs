// Keep deployed icons and mobile SVG sources derived from one original mark.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const master = fs.readFileSync(
  path.join(root, "web/src/assets/homewaf-logo.svg"),
  "utf8",
);
const paths = [...master.matchAll(/<path\b[^>]*\sd="([^"]+)"/g)].map(
  (match) => match[1],
);
if (paths.length !== 1 || !paths[0].startsWith("M"))
  throw new Error("HomeWaf mark must be one absolute compound path");
const silhouette = paths.join(" ");
const brandBlue = master.match(/<path\b[^>]*\sfill="(#[0-9a-f]{6})"/i)?.[1];
if (!brandBlue)
  throw new Error("HomeWaf mark must declare its solid brand color");
const svg = (size, body) =>
  `<!-- Original HomeWaf brand mark. Root LICENSE applies. -->\n<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 ${size} ${size}" role="img" aria-label="HomeWaf">\n${body}\n</svg>\n`;
const cutout = (color) =>
  `<path fill="${color}" fill-rule="evenodd" d="${silhouette}"/>`;
const assets = new Map([
  ["web/public/brand/homewaf-logo.svg", master],
  ["web/public/favicon.svg", master],
  ["cmd/homewaf/exedata/capjs/favicon.svg", master],
  ["cmd/homewaf/exedata/captcha/favicon.svg", master],
  [
    "web/public/brand/homewaf-app-icon.svg",
    svg(
      1024,
      `  <rect id="background" width="1024" height="1024" fill="${brandBlue}"/>\n  <g id="foreground" transform="translate(192 192) scale(10)">${cutout("#fff")}</g>`,
    ),
  ],
  [
    "web/public/brand/homewaf-app-foreground.svg",
    svg(108, `  <g transform="translate(23.92 23.92) scale(0.94)">${cutout("#fff")}</g>`),
  ],
  [
    "web/public/brand/homewaf-app-background.svg",
    svg(108, `  <rect width="108" height="108" fill="${brandBlue}"/>`),
  ],
  [
    "web/public/brand/homewaf-monochrome.svg",
    svg(108, `  <g transform="translate(23.92 23.92) scale(0.94)">${cutout("currentColor")}</g>`),
  ],
]);
for (const [relative, contents] of assets) {
  const destination = path.join(root, relative);
  fs.mkdirSync(path.dirname(destination), { recursive: true });
  const normalized = contents.replace(/\r\n/g, "\n");
  if (
    !fs.existsSync(destination) ||
    fs.readFileSync(destination, "utf8") !== normalized
  ) {
    fs.writeFileSync(destination, normalized);
  }
}
console.log(`HomeWaf: synchronized ${assets.size} SVG brand assets`);
