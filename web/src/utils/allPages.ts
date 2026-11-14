// Selectors and configuration matrices need complete lists while the API bounds
// each response. Reject incomplete reads instead of silently dropping choices.
export async function requestAllPages(fetchPage: (params: any) => Promise<any>, params: Record<string, unknown> = {}) {
  const list: any[] = [];
  for (let pageIndex = 1; pageIndex <= 100; pageIndex += 1) {
    const result = await fetchPage({ ...params, pageIndex, pageSize: 1000 });
    if (result.code !== 0) return result;
    const page = result.data?.list ?? [];
    const total = Number(result.data?.total);
    if (!Array.isArray(page) || !Number.isFinite(total) || total < 0) throw new Error('列表响应无效，请重试');
    list.push(...page);
    if (list.length >= total) return { ...result, data: { ...result.data, list } };
    if (!page.length) throw new Error('列表读取不完整，请重试');
  }
  throw new Error('列表数量超出读取范围，请缩小查询范围');
}
