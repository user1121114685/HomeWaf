package waf_service

import (
	"errors"

	"HomeWaf/internal/model"
	"HomeWaf/internal/platform/utils"
	"HomeWaf/internal/protection/wafpipeline"
	"gorm.io/gorm"
)

// portConfigChange owns the database write and its compensation if runtime activation fails.
type portConfigChange struct {
	next          model.PortConfig
	previous      *model.PortConfig
	createdUserID string
}

func updatePortRevision(tx *gorm.DB, row *model.PortConfig, expected int64) *gorm.DB {
	return tx.Model(&model.PortConfig{}).
		Where("port = ? AND revision = ?", row.Port, expected).
		Select("ConfigJSON", "SecretsJSON", "Revision", "UPDATE_TIME").
		Updates(row)
}

func (change *portConfigChange) commit(db *gorm.DB, req PortSaveRequest) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := defenseTransactionOwner(tx, req.defenseOwner); err != nil {
			return err
		}
		if change.previous == nil {
			if err := tx.Create(&change.next).Error; err != nil {
				return err
			}
		} else {
			result := updatePortRevision(tx, &change.next, change.previous.Revision)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("配置已被其他进程修改，请刷新后重试")
			}
		}
		if req.Username != "" {
			hash, err := utils.BcryptHash(req.Password)
			if err != nil {
				return err
			}
			version, err := pipelineSecret()
			if err != nil {
				return err
			}
			user := model.GateUser{
				BaseOrm: pipelineORM(), PortID: wafpipeline.PortID(uint16(change.next.Port)),
				Username: req.Username, PasswordHash: hash, TOTPSecret: req.InitialTOTP,
				AuthVersion: version, Enabled: true,
			}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
			change.createdUserID = user.Id
		}
		return nil
	})
}

func (change *portConfigChange) restore(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var result *gorm.DB
		if change.previous == nil {
			result = tx.Where("port = ? AND revision = ?", change.next.Port, change.next.Revision).Delete(&model.PortConfig{})
		} else {
			result = updatePortRevision(tx, change.previous, change.next.Revision)
		}
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("配置已被较新请求修改，回滚不会覆盖新版本")
		}
		if change.createdUserID != "" {
			if e := tx.Where("id = ?", change.createdUserID).Delete(&model.GateUser{}).Error; e != nil {
				return e
			}
		}
		return nil
	})
}
