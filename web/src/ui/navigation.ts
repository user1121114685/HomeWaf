export interface NavigationItem {
  name: string;
  path: string;
  title: string;
  group: string;
  newTab: boolean;
}

/** Consume the permitted route tree, never the unfiltered router registry. */
export function navigationItems(routes: any[], translate: (key: string) => string): NavigationItem[] {
  const items: NavigationItem[] = [];
  function visit(nodes: any[], parent = '', group = '') {
    for (const node of nodes || []) {
      if (node.meta?.hidden) continue;
      const path = node.path?.startsWith('/') ? node.path : `${parent}/${node.path}`;
      const title = node.meta?.title ? translate(node.meta.title) : '';
      if (node.children?.length) visit(node.children, path, title || group);
      else if (title && node.name && !node.redirect && !items.some((item) => item.path === path)) {
        items.push({ name: String(node.name), path, title, group, newTab: !!node.meta?.openInNewTab });
      }
    }
  }
  visit(routes);
  return items;
}

export function filterNavigation(items: NavigationItem[], query: string) {
  const words = query.trim().toLocaleLowerCase().split(/\s+/).filter(Boolean);
  return items.filter((item) =>
    words.every((word) => `${item.group} ${item.title} ${item.path}`.toLocaleLowerCase().includes(word)),
  );
}
