# HomeWaf Logo

标志以 Home 的首字母 H 构成盾牌轮廓：上部开口、中央横梁与下部留白合为一个图形，表达家庭服务的访问防护。使用单一复合路径，留白真实透明，不依赖小字、描边、渐变或阴影。

## 源文件与使用场景

| 文件 | 用途 |
| --- | --- |
| `web/src/assets/homewaf-logo.svg` | 唯一编辑源，64 × 64，供 Vue 组件使用 |
| `web/public/brand/homewaf-logo.svg` | 透明底彩色标志，浏览器图标及通用展示 |
| `web/public/brand/homewaf-app-icon.svg` | 1024 × 1024 方形 App 图标源，蓝底、白色标志 |
| `web/public/brand/homewaf-app-foreground.svg` | 108 × 108 透明前景层 |
| `web/public/brand/homewaf-app-background.svg` | 108 × 108 纯色背景层 |
| `web/public/brand/homewaf-monochrome.svg` | 108 × 108 单色透明标志，适合系统主题着色 |

品牌蓝为 `#2563eb`，白色为 `#ffffff`。所有文件均为纯 SVG 矢量，不含位图、字体、脚本或外部资源。

## 手机图标适配

- 方形源文件不预先切圆角，让系统决定最终外形。
- 标志居中并保留裁切空间；圆形、圆角矩形下均保留完整的 H 形轮廓与盾尖。
- 前景和背景分开保存，便于未来适配 Android 自适应图标；单色版保留真实透明留白。
- App 图标不加入 HomeWaf 字样、宣传语或角标，小尺寸下优先保持图形识别。
- 这些文件是设计源。接入原生 App 时，应按目标平台导出图片或转换为平台支持的矢量格式，不能把 SVG 直接当作 Android VectorDrawable。

## 同步资源

编辑主源后运行：

```sh
node scripts/sync-brand.mjs
```

脚本同步网页图标、两套验证页 favicon 及手机 SVG 源文件。不要分别修改派生图标。图案为 HomeWaf 原创设计，适用仓库根目录 LICENSE。
