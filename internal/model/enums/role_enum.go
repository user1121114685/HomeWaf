package enums

// 账号角色
// 国标 GB/T 32917 §7「自身安全」要求权限分离，这里定义四类角色：
//   - superAdmin   超级管理员：拥有全部权限，首次安装时显式授予引导账号
//   - systemAdmin  系统管理员：账户管理 + 系统配置
//   - securityAdmin 安全管理员：WAF 策略/规则/防护配置
//   - auditAdmin   审计管理员：日志审计（只读），独立审计系统员与安全员，三者相互制约
const (
	ROLE_SUPER_ADMIN    = "superAdmin"    // 超级管理员
	ROLE_SYSTEM_ADMIN   = "systemAdmin"   // 系统管理员
	ROLE_SECURITY_ADMIN = "securityAdmin" // 安全管理员
	ROLE_AUDIT_ADMIN    = "auditAdmin"    // 审计管理员
)

// AllRoles 返回全部合法角色
func AllRoles() []string {
	return []string{ROLE_SUPER_ADMIN, ROLE_SYSTEM_ADMIN, ROLE_SECURITY_ADMIN, ROLE_AUDIT_ADMIN}
}

// IsValidRole 判断角色字符串是否合法
func IsValidRole(role string) bool {
	switch role {
	case ROLE_SUPER_ADMIN, ROLE_SYSTEM_ADMIN, ROLE_SECURITY_ADMIN, ROLE_AUDIT_ADMIN:
		return true
	default:
		return false
	}
}
