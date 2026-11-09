import locations from './country-locations.json' with { type: 'json' };
import type { FeatureCollection, LineString, Point } from 'geojson';
import type { StyleSpecification, ExpressionSpecification } from 'maplibre-gl';
import { eventLocation, type Region, type SecurityEvent } from './data.ts';

export type Coordinate = [longitude: number, latitude: number];
export const DEFAULT_CENTER: Coordinate = [104.5, 25];
export const MAX_SCALE = 6;
const aliases: Record<string, string> = locations.aliases;
export const countryLocations: Record<string, { name: string; coordinates: number[] }> = locations.countries;
export function countryId(name: string) {
  return aliases[name.trim().toLowerCase()];
}
export function countryCoordinate(name: string): Coordinate | null {
  const position = countryLocations[countryId(name)]?.coordinates;
  return position ? [position[0], position[1]] : null;
}
export function aggregateRegions(regions: Region[]) {
  const values = new Map<string, number>();
  let unmapped = 0;
  for (const region of regions) {
    if (!Number.isFinite(region.value) || region.value <= 0) continue;
    const id = countryId(region.name);
    if (!id || !countryLocations[id]) {
      unmapped++;
      continue;
    }
    values.set(id, (values.get(id) || 0) + region.value);
  }
  return {
    unmapped,
    mapped: [...values]
      .map(([id, value]) => ({ id, value, ...countryLocations[id] }))
      .sort((a, b) => b.value - a.value),
  };
}
export function sourceIPDetails(id: string, events: SecurityEvent[]) {
  const seen = new Set<string>();
  return events
    .filter((event) => {
      if (countryId(event.country || '') !== id || !event.src_ip || seen.has(event.src_ip)) return false;
      seen.add(event.src_ip);
      return true;
    })
    .slice(0, 8)
    .map((event) => ({ ip: event.src_ip, location: eventLocation(event) }));
}
export function emptyCollection(): FeatureCollection {
  return { type: 'FeatureCollection', features: [] };
}
export function pointCollection(
  points: { coordinates: Coordinate; properties?: Record<string, unknown> }[],
): FeatureCollection<Point> {
  return {
    type: 'FeatureCollection',
    features: points.map(({ coordinates, properties }) => ({
      type: 'Feature',
      properties: properties || {},
      geometry: { type: 'Point', coordinates },
    })),
  };
}
export function lineCollection(lines: Coordinate[][]): FeatureCollection<LineString> {
  return {
    type: 'FeatureCollection',
    features: lines.map((coordinates) => ({
      type: 'Feature',
      properties: {},
      geometry: { type: 'LineString', coordinates },
    })),
  };
}
// Curve in normalized Mercator space. Unwrap longitude so dateline flights take
// the short route; MapLibre renders the same geographic features in world copies.
export function flightCoordinates(from: Coordinate, to: Coordinate): Coordinate[] {
  const mercatorY = (latitude: number) => {
    const radians = (Math.max(-85, Math.min(85, latitude)) * Math.PI) / 180;
    return (1 - Math.log(Math.tan(Math.PI / 4 + radians / 2)) / Math.PI) / 2;
  };
  const wrap = (value: number) => ((((value + 180) % 360) + 360) % 360) - 180;
  const sx = (to[0] + wrap(from[0] - to[0]) + 180) / 360,
    sy = mercatorY(from[1]);
  const tx = (to[0] + 180) / 360,
    ty = mercatorY(to[1]);
  const cx = (sx + tx) / 2,
    cy = Math.max(0.005, Math.min(sy, ty) - Math.min(0.13, Math.hypot(tx - sx, ty - sy) * 0.3));
  return Array.from({ length: 49 }, (_, index) => {
    const t = index / 48,
      a = (1 - t) ** 2,
      b = 2 * (1 - t) * t,
      c = t ** 2;
    const x = a * sx + b * cx + c * tx,
      y = a * sy + b * cy + c * ty;
    return [x * 360 - 180, (Math.atan(Math.sinh(Math.PI * (1 - 2 * y))) * 180) / Math.PI];
  });
}
export function countryFill(dark: boolean): ExpressionSpecification {
  const neutral = dark
    ? ['#26374b', '#2e4256', '#354b60', '#3c5265', '#2c4555']
    : ['#dce5ed', '#cddbe5', '#c2d4df', '#d4dfe8', '#b8cdd9'];
  return [
    'case',
    ['>', ['coalesce', ['feature-state', 'count'], 0], 0],
    [
      'interpolate',
      ['linear'],
      ['coalesce', ['feature-state', 'intensity'], 0],
      0,
      dark ? '#685142' : '#f1d9b6',
      0.4,
      '#e6ac69',
      0.7,
      '#dc7857',
      1,
      '#cd4f4e',
    ],
    ['match', ['get', 'shade'], 0, neutral[0], 1, neutral[1], 2, neutral[2], 3, neutral[3], neutral[4]],
  ];
}
export function createMapStyle(countriesUrl: string, dark: boolean): StyleSpecification {
  return {
    version: 8,
    projection: { type: 'mercator' },
    transition: { duration: 0, delay: 0 },
    // No tiles, remote style, glyph service, graticule, terrain or 3D layer.
    sources: {
      countries: { type: 'geojson', data: countriesUrl, promoteId: 'id', attribution: 'Natural Earth', tolerance: 0.3 },
      origins: { type: 'geojson', data: emptyCollection() },
      flights: { type: 'geojson', data: emptyCollection() },
      'flight-heads': { type: 'geojson', data: emptyCollection() },
    },
    layers: [
      { id: 'ocean', type: 'background', paint: { 'background-color': dark ? '#0b1220' : '#f0f2fa' } },
      {
        id: 'country-fill',
        type: 'fill',
        source: 'countries',
        paint: { 'fill-color': countryFill(dark), 'fill-opacity': 1 },
      },
      {
        id: 'country-borders',
        type: 'line',
        source: 'countries',
        paint: { 'line-color': dark ? '#73859b' : '#839caf', 'line-width': 0.8, 'line-opacity': 0.8 },
      },
      {
        id: 'selected-border',
        type: 'line',
        source: 'countries',
        filter: ['==', ['get', 'id'], ''],
        paint: { 'line-color': '#db9f4d', 'line-width': 2 },
      },
      {
        id: 'china-border',
        type: 'line',
        source: 'countries',
        filter: ['==', ['get', 'id'], 'CHN'],
        layout: { visibility: 'none' },
        paint: { 'line-color': '#df4148', 'line-width': 2 },
      },
      {
        id: 'attack-flights',
        type: 'line',
        source: 'flights',
        layout: { 'line-cap': 'round', 'line-join': 'round' },
        paint: { 'line-color': '#de9b63', 'line-width': 1.4, 'line-opacity': 0.8 },
      },
      {
        id: 'origin-halo',
        type: 'circle',
        source: 'origins',
        paint: { 'circle-radius': 9, 'circle-color': '#d27654', 'circle-opacity': 0.15 },
      },
      {
        id: 'origin-points',
        type: 'circle',
        source: 'origins',
        paint: {
          'circle-radius': 3.5,
          'circle-color': '#b65e46',
          'circle-stroke-color': '#fff0df',
          'circle-stroke-width': 1.2,
        },
      },
      {
        id: 'origin-labels',
        type: 'symbol',
        source: 'origins',
        layout: {
          'text-field': ['get', 'name'],
          // Render glyphs with local system fonts, without a remote glyph URL.
          'text-font': ['Microsoft YaHei', 'PingFang SC', 'Noto Sans CJK SC', 'sans-serif'],
          'text-size': ['interpolate', ['linear'], ['zoom'], 0, 11, 3, 13, 6, 15],
          'text-variable-anchor': ['bottom', 'top', 'left', 'right'],
          'text-radial-offset': 0.85,
          'text-padding': 4,
          'text-max-width': 7,
          'text-allow-overlap': false,
          'text-ignore-placement': false,
          'symbol-sort-key': ['-', 0, ['get', 'count']],
        },
        paint: {
          'text-color': dark ? '#f1e2d1' : '#563b2d',
          'text-halo-color': dark ? '#111c2c' : '#fff8ef',
          'text-halo-width': 1.6,
          'text-halo-blur': 0.4,
        },
      },
      {
        id: 'flight-heads',
        type: 'circle',
        source: 'flight-heads',
        paint: {
          'circle-radius': 3,
          'circle-color': '#ffc990',
          'circle-stroke-color': '#cf7351',
          'circle-stroke-width': 1,
        },
      },
    ],
  };
}
