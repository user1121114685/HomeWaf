// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package ssl

import (
	"HomeWaf/customtype"
	"HomeWaf/internal/dnsproviders"
	"HomeWaf/model"
	"HomeWaf/utils"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns"
	"github.com/go-acme/lego/v4/registration"
)

type MyUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *MyUser) GetEmail() string {
	return u.Email
}
func (u MyUser) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *MyUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

func RegistrationSSL(order model.SslOrder, savePath string, caServerAddress string, applyPlatform string, eab_kid string, eab_hmac_key string) (model.SslOrder, error) {
	var dnsProvider challenge.Provider
	if order.ApplyMethod == "dns01" {
		var err error
		dnsProvider, err = GetDnsProvider(order.ApplyDns)
		if err != nil {
			return order, err
		}
	} else if order.ApplyMethod != "http01" {
		return order, fmt.Errorf("unsupported ACME challenge method: %s", order.ApplyMethod)
	}
	isIpSSL := utils.IsIP(order.ApplyDomain)
	myUser := MyUser{
		Email: order.ApplyEmail,
	}
	if order.ApplyKey == "" {
		// Create a user. New accounts need an email and private key to start.
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return order, err
		}
		toPEMPrivate, err := privateKeyToPEM(privateKey)
		if err != nil {
			return order, err
		} else {
			order.ApplyKey = toPEMPrivate
		}
		myUser.key = privateKey
	} else {
		privateKey, err := pemToPrivateKey(order.ApplyKey)
		if err != nil {
			return order, err
		} else {
			myUser.key = privateKey
		}
	}

	//order.ApplyKey = privateKey
	config := lego.NewConfig(&myUser)

	config.CADirURL = caServerAddress
	config.Certificate.KeyType = certcrypto.RSA2048
	if isIpSSL {
		//是否是ip申请ssl证书
		config.Certificate.DisableCommonName = true
	}

	// A client facilitates communication with the CA server.
	acmeInfo(fmt.Sprintf("%s正在连接 CA 目录 平台=%s", acmeLogPrefix, applyPlatform))
	client, err := lego.NewClient(config)
	if err != nil {
		return order, err
	}

	// We specify an HTTP port of 5002 and an TLS port of 5001 on all interfaces
	// because we aren't running as root and can't bind a listener to port 80 and 443
	// (used later when we attempt to pass challenges). Keep in mind that you still
	// need to proxy challenge traffic to port 5002 and 5001.
	switch order.ApplyMethod {
	case "http01":
		provider, err := newLoggingWebrootProvider(savePath, order.HostCode)
		if err != nil {
			return order, err
		}
		if err = client.Challenge.SetHTTP01Provider(provider); err != nil {
			// 原来这里的 err 被后面的注册流程覆盖掉了，设置失败会静默变成
			// 一次"莫名其妙的校验超时"。不改控制流，但至少要留下证据。
			acmeError(fmt.Sprintf("设置HTTP-01 provider失败 域名=%s 错误=%v", order.ApplyDomain, err))
			return order, err
		}
	case "dns01":
		err = setDNS01ProviderWithRetry(client, newLoggingDNSProvider(dnsProvider, order.ApplyDns), order.SkipDNSVerify)
		if err != nil {
			acmeError(fmt.Sprintf("%sDNS校验-设置provider失败 渠道=%s 错误=%v", acmeLogPrefix, order.ApplyDns, err))
			return order, err
		}
	}

	// New users will need to register
	acmeInfo(fmt.Sprintf("%s正在注册或恢复 CA 账户 平台=%s", acmeLogPrefix, applyPlatform))
	var reg *registration.Resource
	if applyPlatform == "zerossl" {
		// ZeroSSL 需要使用 EAB (External Account Binding) 方式注册
		eabOptions := registration.RegisterEABOptions{
			TermsOfServiceAgreed: true,
			Kid:                  eab_kid,
			HmacEncoded:          eab_hmac_key,
		}
		reg, err = client.Registration.RegisterWithExternalAccountBinding(eabOptions)
		if err != nil {
			return order, err
		}
	} else {
		// 其他平台使用原来的注册方式
		reg, err = client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return order, err
		}
	}
	myUser.Registration = reg

	certificates := &certificate.Resource{}
	if !isIpSSL {
		acmeInfo(fmt.Sprintf("%s开始向CA申请域名证书 域名=%s 校验方式=%s 平台=%s",
			acmeLogPrefix, order.ApplyDomain, order.ApplyMethod, applyPlatform))
		request := certificate.ObtainRequest{
			Domains: strings.Split(order.ApplyDomain, ","),
			Bundle:  true,
		}
		certificatesLocal, err := client.Certificate.Obtain(request)

		if err != nil {
			acmeError(fmt.Sprintf("%s向CA申请域名证书失败 域名=%s 校验方式=%s 错误=%v",
				acmeLogPrefix, order.ApplyDomain, order.ApplyMethod, err))
			return order, err
		}
		certificates = certificatesLocal
	} else {
		// IP 证书：CA 侧走 CSR 方式，且只签发 shortlived（短周期）证书，
		// 有效期只有几天，完全依赖自动续期，所以这条路径的日志比域名证书更重要。
		acmeInfo(fmt.Sprintf("%sIP证书-开始申请 IP=%s 校验方式=%s(IP证书仅支持文件验证) 证书类型=shortlived(短周期) 平台=%s",
			acmeLogPrefix, order.ApplyDomain, order.ApplyMethod, applyPlatform))

		// 1. 为证书生成新的私钥
		certPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			acmeError(fmt.Sprintf("%sIP证书-生成私钥失败 IP=%s 错误=%v", acmeLogPrefix, order.ApplyDomain, err))
			return order, fmt.Errorf("generate cert private key failed: %w", err)
		}

		// 2. 用新私钥创建 CSR
		csrDER, err := x509.CreateCertificateRequest(
			rand.Reader,
			&x509.CertificateRequest{
				Subject: pkix.Name{
					CommonName: "",
				},
				IPAddresses: []net.IP{net.ParseIP(order.ApplyDomain)},
			},
			certPrivateKey, // 使用证书专用私钥
		)
		if err != nil {
			return order, err
		}

		// 3. 解析 CSR
		csr, err := x509.ParseCertificateRequest(csrDER)
		if err != nil {
			return order, err
		}

		// 4. 申请证书
		certificatesLocal, err := client.Certificate.ObtainForCSR(certificate.ObtainForCSRRequest{
			CSR:        csr,
			Bundle:     true,
			PrivateKey: certPrivateKey, //  使用证书专用私钥
			Profile:    "shortlived",
		})
		if err != nil {
			acmeError(fmt.Sprintf("%sIP证书-向CA申请失败 IP=%s 错误=%v", acmeLogPrefix, order.ApplyDomain, err))
			return order, err
		}
		certificates = certificatesLocal
	}
	// Each certificate comes back with the cert bytes, the bytes of the client's
	// private key, and a certificate URL. SAVE THESE TO DISK.

	order.ResultPrivateKey = certificates.PrivateKey
	order.ResultCertificate = certificates.Certificate
	order.ResultCertStableURL = certificates.CertStableURL
	order.ResultCertURL = certificates.CertURL
	order.ResultCSR = certificates.CSR
	// IP证书情况下，certificates.Domain可能为空，使用申请的IP地址
	if isIpSSL && certificates.Domain == "" {
		order.ResultDomain = order.ApplyDomain
	} else {
		order.ResultDomain = certificates.Domain
	}
	order.ResultIssuerCertificate = certificates.IssuerCertificate
	block, _ := pem.Decode(order.ResultCertificate)
	if block != nil {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err == nil {
			order.ResultValidTo = customtype.JsonTime(cert.NotAfter)
			acmeInfo(fmt.Sprintf("%s证书签发成功 申请对象=%s 校验方式=%s 有效期至=%s",
				acmeLogPrefix, order.ApplyDomain, order.ApplyMethod, cert.NotAfter.Format("2006-01-02 15:04:05")))
		} else {
			acmeWarn(fmt.Sprintf("%s证书已签发但解析有效期失败 申请对象=%s 错误=%v", acmeLogPrefix, order.ApplyDomain, err))
		}
	}

	return order, nil
}

func ReNewSSL(order model.SslOrder, savePath string, caServerAddress string, applyPlatform string, eab_kid string, eab_hmac_key string) (model.SslOrder, error) {
	var dnsProvider challenge.Provider
	if order.ApplyMethod == "dns01" {
		var err error
		dnsProvider, err = GetDnsProvider(order.ApplyDns)
		if err != nil {
			return order, err
		}
	} else if order.ApplyMethod != "http01" {
		return order, fmt.Errorf("unsupported ACME challenge method: %s", order.ApplyMethod)
	}
	// 判断是否是IP证书
	isIpSSL := utils.IsIP(order.ApplyDomain)

	myUser := MyUser{
		Email: order.ApplyEmail,
	}
	if order.ApplyKey == "" {
		// Create a user. New accounts need an email and private key to start.
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return order, err
		}
		toPEMPrivate, err := privateKeyToPEM(privateKey)
		if err != nil {
			return order, err
		} else {
			order.ApplyKey = toPEMPrivate
		}
		myUser.key = privateKey
	} else {
		privateKey, err := pemToPrivateKey(order.ApplyKey)
		if err != nil {
			return order, err
		} else {
			myUser.key = privateKey
		}
	}

	//order.ApplyKey = privateKey
	config := lego.NewConfig(&myUser)
	config.CADirURL = caServerAddress
	config.Certificate.KeyType = certcrypto.RSA2048

	if isIpSSL {
		//是否是ip申请ssl证书
		config.Certificate.DisableCommonName = true
	}

	// A client facilitates communication with the CA server.
	acmeInfo(fmt.Sprintf("%s正在连接 CA 目录 平台=%s", acmeLogPrefix, applyPlatform))
	client, err := lego.NewClient(config)
	if err != nil {
		return order, err
	}

	// We specify an HTTP port of 5002 and an TLS port of 5001 on all interfaces
	// because we aren't running as root and can't bind a listener to port 80 and 443
	// (used later when we attempt to pass challenges). Keep in mind that you still
	// need to proxy challenge traffic to port 5002 and 5001.
	if order.ApplyMethod == "http01" {
		// 续期与申请必须走同一个 provider：否则"申请时看得见、续期时看不见"，
		// 而续期恰恰是无人值守的那一次，出问题更需要日志。
		provider, err := newLoggingWebrootProvider(savePath, order.HostCode)
		if err != nil {
			return order, err
		}
		if err = client.Challenge.SetHTTP01Provider(provider); err != nil {
			acmeError(fmt.Sprintf("续期设置HTTP-01 provider失败 域名=%s 错误=%v", order.ApplyDomain, err))
			return order, err
		}
	} else if order.ApplyMethod == "dns01" {
		// 与申请走同一套包装：续期是无人值守的那一次，日志比申请时更要紧
		err = setDNS01ProviderWithRetry(client, newLoggingDNSProvider(dnsProvider, order.ApplyDns), order.SkipDNSVerify)
		if err != nil {
			acmeError(fmt.Sprintf("%s续期-DNS校验-设置provider失败 渠道=%s 错误=%v", acmeLogPrefix, order.ApplyDns, err))
			return order, err
		}
	}
	// New users will need to register
	acmeInfo(fmt.Sprintf("%s正在注册或恢复 CA 账户 平台=%s", acmeLogPrefix, applyPlatform))
	var reg *registration.Resource
	if applyPlatform == "zerossl" {
		// ZeroSSL 需要使用 EAB (External Account Binding) 方式注册
		eabOptions := registration.RegisterEABOptions{
			TermsOfServiceAgreed: true,
			Kid:                  eab_kid,
			HmacEncoded:          eab_hmac_key,
		}
		reg, err = client.Registration.RegisterWithExternalAccountBinding(eabOptions)
		if err != nil {
			return order, err
		}
	} else {
		// 其他平台使用原来的注册方式
		reg, err = client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return order, err
		}
	}
	myUser.Registration = reg

	certificates := &certificate.Resource{}

	if !isIpSSL {
		// 域名证书：使用续期方式
		acmeInfo(fmt.Sprintf("%s续期-开始向CA续期域名证书 域名=%s 校验方式=%s 平台=%s",
			acmeLogPrefix, order.ApplyDomain, order.ApplyMethod, applyPlatform))
		certRes := certificate.Resource{
			Domain:            order.ResultDomain,
			CertURL:           order.ResultCertURL,
			CertStableURL:     order.ResultCertStableURL,
			PrivateKey:        order.ResultPrivateKey,
			Certificate:       order.ResultCertificate,
			IssuerCertificate: order.ResultIssuerCertificate,
			CSR:               order.ResultCSR,
		}
		//构造参数
		certificatesLocal, err := client.Certificate.RenewWithOptions(certRes, &certificate.RenewOptions{
			Bundle: true,
		})
		if err != nil {
			acmeError(fmt.Sprintf("%s续期-向CA续期域名证书失败 域名=%s 校验方式=%s 错误=%v",
				acmeLogPrefix, order.ApplyDomain, order.ApplyMethod, err))
			return order, err
		}
		certificates = certificatesLocal
	} else {
		// IP证书：使用CSR方式重新申请
		acmeInfo(fmt.Sprintf("%s续期-IP证书-开始重新申请 IP=%s 校验方式=%s 证书类型=shortlived(短周期) 平台=%s",
			acmeLogPrefix, order.ApplyDomain, order.ApplyMethod, applyPlatform))

		// 1. 为证书生成新的私钥
		certPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			acmeError(fmt.Sprintf("%s续期-IP证书-生成私钥失败 IP=%s 错误=%v", acmeLogPrefix, order.ApplyDomain, err))
			return order, fmt.Errorf("generate cert private key failed: %w", err)
		}

		// 2. 用新私钥创建 CSR
		csrDER, err := x509.CreateCertificateRequest(
			rand.Reader,
			&x509.CertificateRequest{
				Subject: pkix.Name{
					CommonName: "",
				},
				IPAddresses: []net.IP{net.ParseIP(order.ApplyDomain)},
			},
			certPrivateKey, // 使用证书专用私钥
		)
		if err != nil {
			return order, err
		}

		// 3. 解析 CSR
		csr, err := x509.ParseCertificateRequest(csrDER)
		if err != nil {
			return order, err
		}

		// 4. 申请新证书
		certificatesLocal, err := client.Certificate.ObtainForCSR(certificate.ObtainForCSRRequest{
			CSR:        csr,
			Bundle:     true,
			PrivateKey: certPrivateKey, // 使用证书专用私钥
			Profile:    "shortlived",
		})
		if err != nil {
			acmeError(fmt.Sprintf("%s续期-IP证书-向CA申请失败 IP=%s 错误=%v", acmeLogPrefix, order.ApplyDomain, err))
			return order, err
		}
		certificates = certificatesLocal
	}

	// Each certificate comes back with the cert bytes, the bytes of the client's
	// private key, and a certificate URL. SAVE THESE TO DISK.

	order.ResultPrivateKey = certificates.PrivateKey
	order.ResultCertificate = certificates.Certificate
	order.ResultCertStableURL = certificates.CertStableURL
	order.ResultCertURL = certificates.CertURL
	order.ResultCSR = certificates.CSR
	// IP证书情况下，certificates.Domain可能为空，使用申请的IP地址
	if isIpSSL && certificates.Domain == "" {
		order.ResultDomain = order.ApplyDomain
	} else {
		order.ResultDomain = certificates.Domain
	}
	order.ResultIssuerCertificate = certificates.IssuerCertificate
	block, _ := pem.Decode(order.ResultCertificate)
	if block != nil {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err == nil {
			order.ResultValidTo = customtype.JsonTime(cert.NotAfter)
			acmeInfo(fmt.Sprintf("%s续期-证书续期成功 申请对象=%s 校验方式=%s 有效期至=%s",
				acmeLogPrefix, order.ApplyDomain, order.ApplyMethod, cert.NotAfter.Format("2006-01-02 15:04:05")))
		} else {
			acmeWarn(fmt.Sprintf("%s续期-证书已签发但解析有效期失败 申请对象=%s 错误=%v", acmeLogPrefix, order.ApplyDomain, err))
		}
	}
	return order, nil
}

// 将ECDSA私钥编码为PEM格式的字符串
func privateKeyToPEM(privateKey *ecdsa.PrivateKey) (string, error) {
	// 将私钥转换为DER格式字节
	privBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return "", err
	}

	// 将DER格式字节封装为PEM格式
	pemKey := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privBytes,
	})

	return string(pemKey), nil
}

// 将PEM格式字符串转换回ECDSA私钥
func pemToPrivateKey(pemStr string) (*ecdsa.PrivateKey, error) {
	// 解码PEM字符串
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil || block.Type != "EC PRIVATE KEY" {
		return nil, fmt.Errorf("invalid PEM block")
	}

	// 解析DER格式的私钥
	privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

func GetDnsProvider(dnsName string) (challenge.Provider, error) {
	if err := dnsproviders.ValidateProvider(dnsName); err != nil {
		return nil, err
	}
	provider, err := dns.NewDNSChallengeProviderByName(dnsName)
	if err != nil {
		return nil, fmt.Errorf("%s", dnsproviders.Redact(err.Error()))
	}
	return provider, nil
}

// Certificate DNS lookups use the operator-selected domestic resolver. This
// changes ACME DNS verification only; it does not change the host's OS DNS.
const certificateDNSResolver = "223.5.5.5:53"

func setDNS01ProviderWithRetry(client *lego.Client, dnsProvider challenge.Provider, skipDNSVerify int64) error {
	options := []dns01.ChallengeOption{
		dns01.AddRecursiveNameservers([]string{certificateDNSResolver}),
		dns01.AddDNSTimeout(5 * time.Second),
	}
	acmeInfo(fmt.Sprintf("%sDNS 解析器=%s，单次 DNS 网络查询超时=5秒", acmeLogPrefix, certificateDNSResolver))
	if skipDNSVerify == 1 {
		const propagationWait = 180 * time.Second
		acmeInfo(fmt.Sprintf("%s已跳过本地传播检查，等待 %v 后由 CA 验证", acmeLogPrefix, propagationWait))
		options = append(options, dns01.PropagationWait(propagationWait, true))
	} else {
		acmeInfo(acmeLogPrefix + "启用 DNS 传播检查，轮询间隔与总超时遵循服务商设置")
		options = append(options, dns01.WrapPreCheck(logDNSPropagationCheck))
	}
	return client.Challenge.SetDNS01Provider(dnsProvider, options...)
}

// Lego already polls this callback within the provider's propagation timeout.
// An inner retry/sleep loop would multiply that timeout and hide live progress.
func logDNSPropagationCheck(domain, fqdn, value string, check dns01.PreCheckFunc) (bool, error) {
	acmeInfo(fmt.Sprintf("%s正在检查 DNS 传播 域名=%s 记录=TXT %s", acmeLogPrefix, domain, fqdn))
	ok, err := check(fqdn, value)
	if ok && err == nil {
		acmeInfo(fmt.Sprintf("%sDNS 传播检查通过 域名=%s 记录=TXT %s", acmeLogPrefix, domain, fqdn))
	} else {
		acmeWarn(fmt.Sprintf("%sDNS 传播本轮未通过 域名=%s 记录=TXT %s 错误=%v；将按服务商轮询设置继续检查", acmeLogPrefix, domain, fqdn, err))
	}
	return ok, err
}
