# 出站连接、内网放行与代理

## 配置位置

部署配置文件为运行目录下的 `conf/config.yml`。仓库开发实例使用
`.cache/dev/runtime/conf/config.yml`。内网例外项是：

```yaml
security:
  outbound_allowed_hosts: "192.168.1.20,notify.home.lan"
```

这是逗号分隔的主机名、IP 或 CIDR 字符串。主机名精确匹配，不支持 `*`；不填协议、
路径、密钥或端口。已有 `security` 配置块时只修改其中的字段，不要另建重复块。
修改部署文件后重启 HomeWaf 加载。该项不在数据库“系统配置”编辑页中。

**此项是访问内网/环回/保留地址的例外清单，不是公网域名必须逐项登记的清单。**
公网目标默认允许。官方 Bark、钉钉、飞书等 HTTP 通知地址不需要逐个添加；自建在
内网的 Bark、Webhook、SMTP/Socket 服务才需要明确声明相应主机。
填入白名单不会修复 DNS 解析失败、网络超时或 TLS 信任问题。

## ZeroSSL 与 Let’s Encrypt

- ZeroSSL 自动获取 EAB 访问 `api.zerossl.com`。
- ZeroSSL ACME 目录访问 `acme.zerossl.com`。
- Let’s Encrypt 生产 ACME 目录访问 `acme-v02.api.letsencrypt.org`。

这些请求需要可用的出站 DNS 和 HTTPS，不要求为 DNS-01 开放公网入站 80/443。
证书机构请求并非被 `security.outbound_allowed_hosts` 的空值默认禁用；DNS 服务商
API 也需要单独可达。使用自建通知服务时，目标端口取决于其 HTTP、SMTP 或 Socket 配置。

## 代理配置

ZeroSSL 认证、lego 的 ACME 客户端和 HTTP 通知客户端读取 HomeWaf 进程的
`HTTPS_PROXY`、`HTTP_PROXY`、`NO_PROXY` 环境变量。必须配置在启动 HomeWaf 的进程、
系统服务或容器环境中，再重启该进程；仅给浏览器配置代理不会自动应用到 Go 服务。
只设置 `ALL_PROXY` 不能替代 Go HTTP 客户端使用的 `HTTPS_PROXY`。

请求诊断按原因区分 DNS 解析失败、TLS 证书校验失败、超时、代理连接失败和内网目标
被策略拒绝。错误只显示目标主机，不显示 API Key、通知路径、查询参数或原始响应体。

## 本次只读检测

2026-11-14 在当前 Windows 开发机使用 Go DNS/HTTP 客户端、无账号凭据进行检测：

| 主机 | 结果 |
| --- | --- |
| `api.zerossl.com` | Go DNS 返回 `no such host`，HTTPS 无法建立 |
| `acme.zerossl.com` | DNS 和 TLS/HTTPS 连通；根路径返回 404，不表示证书申请失败 |
| `acme-v02.api.letsencrypt.org` | DNS 和基础 HTTPS 请求成功 |
| `api.day.app` | DNS 和基础 HTTPS 请求成功 |

该结果是检测时的连通性快照，不证明真实通知发送或证书签发成功。没有发送通知、
提交真实证书订单、读取账号密钥，或修改 DNS、防火墙、代理及内网放行配置。

如果已经取得 ZeroSSL 的完整 EAB Key ID/HMAC Key，可以在认证弹窗选择“直接配置 EAB
凭据”，跳过自动获取 EAB 的 API 请求；ACME 与 DNS 服务商的网络连接仍需正常。
