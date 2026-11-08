// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

export type Module<S = any, R = any> = {
  namespaced?: boolean;
  state: S | (() => S);
  getters?: any;
  mutations?: any;
  actions?: any;
};
function entries(ns: any, values?: any): [string, [string, any][]] {
  const prefix = typeof ns === 'string' ? ns.replace(/\/$/, '') : '';
  const map = typeof ns === 'string' ? values : ns;
  return [prefix, Array.isArray(map) ? map.map((n) => [n, n]) : Object.entries(map || {})];
}
export function mapState(ns: any, values?: any) {
  const [p, m] = entries(ns, values);
  return Object.fromEntries(
    m.map(([n, v]) => [
      n,
      function (this: any) {
        const s = p ? this.$store.state[p] : this.$store.state;
        return typeof v === 'function' ? v.call(this, s, this.$store.getters) : s[v];
      },
    ]),
  );
}
export function mapGetters(ns: any, values?: any) {
  const [p, m] = entries(ns, values);
  return Object.fromEntries(
    m.map(([n, v]) => [
      n,
      function (this: any) {
        return this.$store.getters[(p ? p + '/' : '') + v];
      },
    ]),
  );
}
function commands(method: string, ns: any, values?: any) {
  const [p, m] = entries(ns, values);
  return Object.fromEntries(
    m.map(([n, v]) => [
      n,
      function (this: any, payload: any) {
        return this.$store[method]((p ? p + '/' : '') + v, payload);
      },
    ]),
  );
}
export const mapMutations = (ns: any, v?: any) => commands('commit', ns, v);
export const mapActions = (ns: any, v?: any) => commands('dispatch', ns, v);
