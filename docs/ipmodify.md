<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

# IP 地区库

HomeWaf 内置 IPv4 的 `ip2region.xdb`。IPv6、GeoLite2 和其他地区库由使用者自行提供。

## 数据文件

将文件放到 HomeWaf 程序目录下的 `data/` 中，保持文件名不变：

| 文件名 | 用途 | 是否内置 |
| --- | --- | --- |
| `ip2region.xdb` | IPv4 地区 | 是 |
| `ip2region_v6.xdb` | IPv6 地区 | 否 |
| `GeoLite2-Country.mmdb` | IPv4 / IPv6 国家 | 否 |
| `iplocation.ipdb` | IPv4 / IPv6 地区 | 否 |

`data/` 中的同名数据优先于内置数据。替换前请备份，随后通过管理端「IP 库管理」重新加载，并使用「测试 IP 地址」确认结果。数据来源、获取方式和使用范围以数据提供方的说明为准。

内置 IPv4 数据的来源及校验值记录在 [provenance.json](modernization/provenance.json) 的 `ip2region-provenance` 项中。

## 地区规则

不同数据源可能返回中文或英文国家名。设置规则前，先检查所用数据实际返回的值，避免将 `中国` 和 `China` 当成不同地区而误拦。

例如，规则需要同时允许这两个名称时：

```text
rule Roverseas "海外访问拦截" {
    when MF.COUNTRY != "中国" && MF.COUNTRY != "China"
    then RF.Deny();
}
```

更新或切换数据库后，请重新验证相关规则。历史版本的压测结果、地区名称和数据库格式不能代表当前数据源。
