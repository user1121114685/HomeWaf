// Natural Earth countries/populated places (public domain), bundled for offline maps.
// Run from web/: node scripts/generate-screen-countries.mjs <countries.geojson> [cities.geojson]
import fs from 'node:fs';
const source = process.argv[2];
if (!source) throw new Error('Provide the Natural Earth countries GeoJSON path');
const geo = JSON.parse(fs.readFileSync(source, 'utf8'));
const output = new URL('../src/pages/dashboard/screen/', import.meta.url);
const names = [
  new Intl.DisplayNames(['zh-Hans'], { type: 'region' }),
  new Intl.DisplayNames(['en'], { type: 'region' }),
];
const aliases = {},
  countries = {};
const normalize = (name) => name.trim().toLowerCase();
const rounded = (coordinates) => coordinates.map((c) => (Array.isArray(c) ? rounded(c) : Number(c.toFixed(4))));
const features = geo.features.map((f) => {
  const p = f.properties;
  const id = p.ADM0_A3;
  const iso = /^[A-Z]{2}$/.test(p.ISO_A2_EH) ? p.ISO_A2_EH : p.ISO_A2;
  const displayNames = /^[A-Z]{2}$/.test(iso) ? names.map((n) => n.of(iso)) : [];
  for (const name of [
    p.NAME_ZH,
    p.NAME_ZHT,
    p.NAME,
    p.NAME_EN,
    p.ADMIN,
    p.FORMAL_EN,
    p.ISO_A2,
    p.ISO_A2_EH,
    id,
    ...displayNames,
  ])
    if (name && name !== '-99') aliases[normalize(name)] = id;
  const name = displayNames[0] || p.NAME_ZH || p.NAME;
  if (Number.isFinite(p.LABEL_X) && Number.isFinite(p.LABEL_Y))
    countries[id] = { name, coordinates: [p.LABEL_X, p.LABEL_Y].map((v) => Number(v.toFixed(4))) };
  return {
    type: 'Feature',
    id,
    properties: { id, name, shade: Math.max(0, (p.MAPCOLOR7 || 1) - 1) % 5 },
    geometry: { type: f.geometry.type, coordinates: rounded(f.geometry.coordinates) },
  };
});
for (const name of ['中国大陆', 'mainland china']) aliases[name] = 'CHN';
fs.writeFileSync(
  new URL('world-countries.geojson', output),
  JSON.stringify({ type: 'FeatureCollection', features }) + '\n',
);
fs.writeFileSync(new URL('country-locations.json', output), JSON.stringify({ aliases, countries }) + '\n');
if (process.argv[3]) {
  const cityGeo = JSON.parse(fs.readFileSync(process.argv[3], 'utf8'));
  const countryAliases = {};
  for (const f of geo.features) {
    const p = f.properties;
    const iso = /^[A-Z]{2}$/.test(p.ISO_A2_EH) ? p.ISO_A2_EH : p.ISO_A2;
    if (!/^[A-Z]{2}$/.test(iso)) continue;
    for (const name of [
      iso,
      p.ADM0_A3,
      p.NAME_ZH,
      p.NAME_ZHT,
      p.NAME,
      p.NAME_EN,
      p.ADMIN,
      ...names.map((n) => n.of(iso)),
    ])
      if (name) countryAliases[name] = iso;
  }
  const cities = cityGeo.features
    .filter((f) => /^[A-Z]{2}$/.test(f.properties.ISO_A2))
    .map((f) => {
      const p = f.properties;
      return [
        p.ISO_A2,
        [...new Set([p.NAME, p.NAMEASCII, p.NAME_EN, p.NAME_ZH, p.NAME_ZHT].filter(Boolean))],
        p.ADM1NAME || '',
        ...f.geometry.coordinates.map((v) => Number(v.toFixed(4))),
      ];
    });
  fs.writeFileSync(
    new URL('city-positions.json', output),
    JSON.stringify({ countries: countryAliases, cities }) + '\n',
  );
  console.log(`Generated ${cities.length} country-qualified city locations.`);
}
console.log(`Generated ${features.length} country polygons and ${Object.keys(aliases).length} aliases for MapLibre.`);
