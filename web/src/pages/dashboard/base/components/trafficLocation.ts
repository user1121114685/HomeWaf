import countryLocations from '../../screen/country-locations.json' with { type: 'json' };

type TrafficLocation = { country?: string; province?: string; city?: string; ip_belong?: string };

export function trafficLocation(row: TrafficLocation, locale = 'zh_CN') {
  const zh = locale.startsWith('zh');
  const clean = (value?: string) => {
    const text = String(value || '').trim();
    return ['', '0', '未知', 'unknown'].includes(text.toLowerCase()) ? '' : text;
  };
  const rawCountry = clean(row.country);
  if (!rawCountry && row.country === undefined && row.ip_belong) return row.ip_belong;
  const aliases: Record<string, string> = countryLocations.aliases;
  const countries: Record<string, { name: string }> = countryLocations.countries;
  const country = (zh ? countries[aliases[rawCountry.toLowerCase()]]?.name : '') || rawCountry;
  const parts = [country || (zh ? '国家未知' : 'Unknown country'), clean(row.province)];
  parts.push(clean(row.city) || (zh ? '城市未知' : 'Unknown city'));
  return [...new Set(parts.filter(Boolean))].join(' · ');
}

export function trafficRegions(rows: (TrafficLocation & { name: string; value: number })[], locale = 'zh_CN') {
  const totals = new Map<string, number>();
  for (const row of rows) {
    const country = (row.country || row.name).trim();
    if (!country || country === '0' || country === '内网' || row.value <= 0) continue;
    const name = trafficLocation({ ...row, country }, locale);
    totals.set(name, (totals.get(name) || 0) + row.value);
  }
  return [...totals].map(([name, value]) => ({ name, value })).sort((a, b) => b.value - a.value);
}
