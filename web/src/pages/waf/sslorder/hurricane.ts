export interface HurricaneCredential {
  domain: string;
  token: string;
}

export function serializeHurricaneCredentials(rows: HurricaneCredential[]): string {
  const seen = new Set<string>();
  return rows
    .map(({ domain, token }) => {
      const name = domain.trim().toLowerCase().replace(/\.$/, '');
      const value = token.trim();
      if (
        !name.includes('.') ||
        name.length > 253 ||
        name.split('.').some((label) => !/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label))
      ) {
        throw new Error('请填写不含 *. 或 _acme-challenge. 的域名，例如 example.com。');
      }
      if (!value || /[,:\s]/.test(value)) throw new Error('请填写有效的 DDNS Key，不能包含逗号、冒号或空白。');
      if (seen.has(name)) throw new Error('同一域名只填写一次；根域名和泛域名共用密钥。');
      seen.add(name);
      return `${name}:${value}`;
    })
    .join(',');
}
