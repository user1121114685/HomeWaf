# Simple IP Blocker 示例插件

演示 HomeWaf 的 `IPFilterPlugin` 与 `WafCheckPlugin` 接口，根据配置的 IP 列表返回放行或阻断结果。

## 构建与测试

在仓库根目录执行：

```sh
go build -o artifacts/simple_ip_blocker ./plugins/builtin/simple_ip_blocker
go test ./plugins/builtin/simple_ip_blocker
```

Windows 构建时可将输出文件名改为 `artifacts/simple_ip_blocker.exe`。

## 加载要求

HomeWaf 的进程插件需要签名。请使用受信任的发行方密钥为编译后的插件生成签名，并按[商业授权与插件说明](../../../docs/modernization/MIGRATION.md#商业授权与插件)配置发行构建的可信公钥。

仅复制二进制文件不会绕过签名验证；默认空信任库不接受插件签名。

## 参数

| 参数 | 类型 | 用途 |
| --- | --- | --- |
| `blocked_ips` | 字符串数组 | 需要屏蔽的 IP 列表 |
| `block_reason` | 字符串 | 阻断原因 |

参数处理与默认行为以同目录的实现和测试为准。示例用于插件开发，请在隔离环境验证后再部署。
