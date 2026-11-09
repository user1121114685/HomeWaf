import data from './city-positions.json';

type City = [country: string, names: string[], region: string, longitude: number, latitude: number];
function normalize(value: string) {
  return value
    .normalize('NFKD')
    .replace(/\p{M}/gu, '')
    .toLowerCase()
    .replace(/市$/, '')
    .replace(/[\s\p{P}]/gu, '');
}
const countries = new Map(Object.entries(data.countries).map(([name, iso]) => [normalize(name), iso]));
const cities = new Map<string, City[]>();
for (const city of data.cities as City[]) {
  for (const name of new Set(city[1].map(normalize))) {
    const key = city[0] + '|' + name;
    cities.set(key, [...(cities.get(key) || []), city]);
  }
}
export function locateCity(country: string, city: string, region = '') {
  const iso = countries.get(normalize(country));
  if (!iso || !city) return null;
  const matches = cities.get(iso + '|' + normalize(city)) || [];
  const regional = matches.filter((c) => normalize(c[2]) === normalize(region));
  const unique = regional.length === 1 ? regional : matches;
  // Never choose an arbitrary city when multiple cities share a name.
  if (unique.length !== 1) return null;
  return { longitude: unique[0][3], latitude: unique[0][4] };
}
