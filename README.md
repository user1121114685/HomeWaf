<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

# HomeWaf

HomeWaf 是基于 SamWaf 的独立 Web 应用防火墙修改发行版。Go 主模块为 `HomeWaf`，入口为 `cmd/homewaf`，前端包为 `homewaf-web`。

## 使用许可

**仅非商业用途免费，商业用途必须事先获得书面商业授权，包括公司内部部署。** HomeWaf 自有新增代码及修改使用根目录 `LICENSE` 中的源码可用许可；这不是 OSI 定义的开源许可。SamWaf、SamWafWeb 与其他第三方的原有许可权利保持不变。原始 Apache 授权不能通过改名被撤销，也不能阻止他人独立使用原项目商用。

- [许可全文](LICENSE)、[来源声明](NOTICE)、[第三方清单](THIRD_PARTY_NOTICES.md)
- [合规审查与整改](docs/modernization/COMPLIANCE.md)
- [迁移及发布说明](docs/modernization/MIGRATION.md)
- [实际验证记录](docs/modernization/STATUS.md)
- [依赖 SBOM](docs/modernization/sbom.cdx.json)

## 构建

安装 Go 1.27.3、Node 26.13.0，以及支持 CGO 的 C 编译器。前端源码和锁文件在本仓库；不再下载原项目预编译网页。

```sh
node scripts/build.mjs
node scripts/verify.mjs
```

`build.mjs` 执行锁文件安装、Vue/TypeScript 检查、前端构建及 Go 编译，并把许可声明嵌入网页。开发前端可使用 `npm run dev --prefix web`，后端地址通过 `web/.env` 的 `HOMEWAF_BACKEND` 指定。

容器从本仓库源码构建：`docker compose build && docker compose up -d`。默认管理端只映射到宿主机 127.0.0.1:26666，请通过安全隧道访问或明确配置访问控制。生产数据在 conf、data、ssl、logs 卷中。不要同时启动新旧 WAF 服务占用同一端口或数据库。

## 授权与隐私

管理后台「系统设置 → HomeWaf 授权」提供实例 ID、签名文件验证和确认导入。发布方须离线保管 Ed25519 私钥，仅把可信公钥写入发行构建。默认空信任库不接受任何商业授权或插件签名；不会伪造激活状态。未来云授权服务尚未实现，本次提供离线签名机制和明确的特性校验接口，不虚构服务器、收费价格或合同主体。

默认禁用原项目的更新、公告、公网 IP 查询和机器人数据镜像。发行方可显式配置自己的 HTTPS 服务。详细数据流、配置与限制见迁移文档。

## 上游来源

后端来源：samwafgo/SamWaf，提交 `8d60305a6f5392609f39a397c26ee67e758081f7`。
前端来源：samwafgo/SamWafWeb，提交 `c92ee05f84e58f27fa84beacad28071f0ba5931b`。
保留作者版权、来源和 Apache-2.0 文本；HomeWaf 不代表原项目，不使用其客服、公众号或发布渠道。
