<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <main class="help">
    <h1>HomeWaf 使用与许可说明</h1>
    <p>HomeWaf 是基于 SamWaf 的独立修改发行版，与原项目不存在官方支持或授权背书关系。</p>
    <h2>许可边界</h2>
    <p>HomeWaf 自有新增代码及修改采用非商业源码可用许可。非商业用途免费；公司内部部署、业务防护、收费托管、转售及其他商业用途均需事先取得书面商业授权。这不属于 OSI 定义的开源许可。</p>
    <p>原项目和第三方组件继续适用各自许可；本产品条款不撤销其原有授权。</p>
    <p><a href="/legal/LICENSE" target="_blank" rel="noopener">产品许可全文</a> · <a href="/legal/NOTICE" target="_blank" rel="noopener">第三方声明</a> · <a href="/legal/THIRD_PARTY_NOTICES.md" target="_blank" rel="noopener">依赖清单与许可</a></p>
    <h2>安装与升级</h2>
    <p>发布前请备份 conf、data、ssl、plugins 目录及授权文件。沿用既有数据库与协议兼容标识；不要同时运行新旧服务。详细步骤见发行包 docs/modernization/MIGRATION.md。</p>
    <p>管理端初始口令由本机首次启动生成。首次登录后请修改密码并限制管理端访问范围。后台 API 权限以服务端校验为准。</p>
    <h2>商业授权</h2>
    <p>从发行方向本实例申请签名授权文件，在授权管理中导入。源码仓库不包含生产签名私钥。许可价格、主体、有效期限与服务范围以双方书面合同为准。</p>
    <h2>隐私与更新</h2>
    <p>默认不向原项目服务发送更新、公告、机器识别码或公网 IP 查询。发行方可显式配置自己的 HTTPS 更新端点。主动启用 DNS、证书申请、通知或 AI 服务会向对应服务商发送实现功能所需的数据。</p>
    <p>本地开发与发布说明、迁移细节、依赖版本及已知限制均在源码 docs/modernization 目录中。</p>
    <p><a href="/#/login">返回登录</a></p>
  </main>
</template>
<style scoped>
.help { max-width: 840px; margin: 40px auto; padding: 24px; line-height: 1.8; font-family: system-ui, sans-serif; }
h2 { margin-top: 28px; } a { color: #1d4ed8; }
</style>
