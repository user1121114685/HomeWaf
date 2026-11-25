package waf_service

import (
	"context"
	"math"
	"net"
	"net/netip"
	"strconv"
	"time"

	"HomeWaf/internal/protection/camouflage"
	"HomeWaf/internal/protection/wafpipeline"
)

type AssessmentProfile struct {
	camouflage.ProfileConfig
	Running   bool   `json:"running"`
	LastError string `json:"last_error,omitempty"`
	Predicted int    `json:"predicted"`
	Local     int    `json:"local"`
}

func assessmentReadyProfiles(config camouflage.DefenseConfig, status []camouflage.ProfileStatus) []AssessmentProfile {
	rows := []AssessmentProfile{}
	for _, profile := range config.Profiles {
		row := AssessmentProfile{ProfileConfig: profile}
		for _, s := range status {
			if s.ID == profile.ID && s.Port == profile.Port {
				row.Running = profile.Enabled && s.Enabled && s.Running
				row.LastError = s.LastError
			}
		}
		rows = append(rows, row)
	}
	return rows
}
func AssessmentCamouflage(report *wafpipeline.AssessmentReport, sources []wafpipeline.AssessmentSource, selected string) []AssessmentProfile {
	rows := assessmentReadyProfiles(WafCamouflageServiceApp.Config(), WafCamouflageServiceApp.Status())
	ready := false
	local := WafCamouflageServiceApp.LocalCIDRs()
	for index := range rows {
		row := &rows[index]
		ready = ready || row.Running
		if selected != "" && row.ID != selected {
			continue
		}
		for _, source := range sources {
			ip, err := netip.ParseAddr(source.IP)
			if err != nil {
				continue
			}
			hit := camouflage.Preview(row.ProfileConfig, ip, []byte("GET / HTTP/1.1\r\nUser-Agent: sqlmap/1.8\r\n\r\n"), local)
			if hit.LocalLAN {
				row.Local++
			} else if row.Running {
				row.Predicted++
			}
		}
	}
	report.DirectScore = report.Score
	if ready {
		report.CamouflageScore = 10
	} else {
		report.Advice = append(report.Advice, "建议至少添加一个伪装画像，为它单独选择协议、端口和版本，保存并启用后确认监听运行，用于诱捕并封禁尝试探测的 IP。")
	}
	if report.DirectScore != nil {
		score := int(math.Round(float64(*report.DirectScore)*0.9)) + report.CamouflageScore
		report.Score = &score
	}
	report.Limits = append(report.Limits, "总分 = 业务直连防御分 × 90% + 伪装画像就绪分（0 或 10）。至少一个已保存、启用且监听运行的画像获得 10 分；堆叠画像不重复加分。就绪分不证明公网可达或系统防火墙封禁成功。",
		"画像联动预测以来源先触及画像端口为前提；本轮不发送画像探针、不执行实际封禁，局域网豁免仍生效。每条画像独立选择协议、端口与版本。")
	return rows
}

type AssessmentListener struct {
	Family    string `json:"family"`
	Expected  bool   `json:"expected"`
	Connected bool   `json:"connected"`
	Detail    string `json:"detail"`
}

// Dial only loopback at the saved service port, never user-supplied remote hosts.
func ProbeAssessmentListeners(ctx context.Context, config wafpipeline.PortConfig) []AssessmentListener {
	rows := []AssessmentListener{}
	for _, family := range []string{"IPv4", "IPv6"} {
		host, network, excluded := "127.0.0.1", "tcp4", "ipv6"
		if family == "IPv6" {
			host, network, excluded = "::1", "tcp6", "ipv4"
		}
		row := AssessmentListener{Family: family, Expected: config.ListenEnabled && config.Policy.Network.ListenIPVersion != excluded}
		dialer := net.Dialer{Timeout: 500 * time.Millisecond}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(host, strconv.Itoa(int(config.Port))))
		if err == nil {
			row.Connected = true
			row.Detail = "本机 TCP 连接成功"
			_ = conn.Close()
		} else {
			row.Detail = "本机 TCP 连接未成功（不等同于公网不可达）"
		}
		rows = append(rows, row)
	}
	return rows
}
