# HomeWaf Web

HomeWaf 管理前端，使用 Vue 3、Element Plus、Pinia 和 Vite。

## 本地开发

使用 `package.json` 指定的 Node.js 版本，在本目录执行：

```sh
npm ci
npm run dev
```

通过 `.env` 中的 `HOMEWAF_BACKEND` 设置后端地址。

## 检查与构建

```sh
npm run typecheck
npm test
npm run build
```

如需将前端嵌入 HomeWaf，请在仓库根目录运行 `node scripts/build.mjs`。安装、更新与安全边界见[项目 README](../README.md)。

品牌资源由根目录 `scripts/sync-brand.mjs` 同步；修改主源后运行该脚本，不分别修改派生图标。页面复用 `src/ui/` 的控件与 tokens，工程约束见 [AGENTS.md](../AGENTS.md)。

许可与来源见根目录 [LICENSE](../LICENSE)、[NOTICE](../NOTICE) 和[第三方清单](../THIRD_PARTY_NOTICES.md)。
