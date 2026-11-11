package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// RotateServiceToken returns a bearer token once; only its hash is persisted in
// the encrypted port secret envelope and no policy/template export contains it.
func (s *WafPipelineService) RotateServiceToken(port uint16, revoke bool, operator string, expectedRevision ...int64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := pipelineDB()
	if err != nil {
		return "", err
	}
	var row model.PortConfig
	if err = db.Where("port = ?", port).First(&row).Error; err != nil {
		return "", err
	}
	if len(expectedRevision) > 0 && expectedRevision[0] > 0 && row.Revision != expectedRevision[0] {
		return "", errors.New("端口配置已更新，请刷新后再轮换令牌")
	}
	snap, err := decodePort(row)
	if err != nil {
		return "", err
	}
	token := ""
	snap.Secrets.ServiceTokenHashes = nil
	if !revoke {
		token, err = pipelineSecret()
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256([]byte(token))
		snap.Secrets.ServiceTokenHashes = []string{hex.EncodeToString(sum[:])}
	}
	snap.Config.Revision++
	snap.PublishedAt = time.Now()
	if s.Prepare != nil {
		if err = s.Prepare(snap); err != nil {
			return "", err
		}
	}
	config, _ := json.Marshal(snap.Config)
	secret, _ := json.Marshal(snap.Secrets)
	row.ConfigJSON = string(config)
	row.SecretsJSON = string(secret)
	row.Revision = snap.Config.Revision
	if err = db.Save(&row).Error; err != nil {
		return "", err
	}
	if err = wafpipeline.Ports.Publish(port, func() (*wafpipeline.Snapshot, error) { return snap, nil }); err != nil {
		return "", err
	}
	s.audit(operator, port, "gate_service_token_rotated")
	return token, nil
}
