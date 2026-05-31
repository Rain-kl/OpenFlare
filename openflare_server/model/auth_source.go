package model

import (
	"errors"
	"openflare/utils"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	AuthSourceTypeGitHub = "github"
	AuthSourceTypeOIDC   = "oidc"
)

var authSourceNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`)

type AuthSource struct {
	ID                     uint      `json:"id"`
	Name                   string    `json:"name" gorm:"uniqueIndex;size:80;not null"`
	Type                   string    `json:"type" gorm:"index;size:20;not null"`
	DisplayName            string    `json:"display_name" gorm:"size:100"`
	IsActive               bool      `json:"is_active" gorm:"index;not null;default:false"`
	ClientID               string    `json:"client_id" gorm:"column:client_id;size:255"`
	ClientSecret           string    `json:"-" gorm:"column:client_secret;size:1024"`
	OpenIDDiscoveryURL     string    `json:"openid_discovery_url" gorm:"column:openid_discovery_url;size:1024"`
	Scopes                 string    `json:"scopes" gorm:"size:255"`
	IconURL                string    `json:"icon_url" gorm:"size:1024"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
	ClientSecretConfigured bool      `json:"client_secret_configured" gorm:"-"`
}

type ExternalAccount struct {
	ID               uint       `json:"id"`
	AuthSourceID     uint       `json:"auth_source_id" gorm:"uniqueIndex:idx_external_account_source_external;index;not null"`
	UserID           int        `json:"user_id" gorm:"index;not null"`
	ExternalID       string     `json:"external_id" gorm:"uniqueIndex:idx_external_account_source_external;size:255;not null"`
	ExternalUsername string     `json:"external_username" gorm:"size:255"`
	Email            string     `json:"email" gorm:"size:255"`
	AuthSource       AuthSource `json:"-" gorm:"constraint:OnDelete:CASCADE"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ExternalAccountView struct {
	ID               uint      `json:"id"`
	AuthSourceID     uint      `json:"auth_source_id"`
	AuthSourceName   string    `json:"auth_source_name"`
	AuthSourceType   string    `json:"auth_source_type"`
	AuthSourceLabel  string    `json:"auth_source_label"`
	ExternalUsername string    `json:"external_username"`
	Email            string    `json:"email"`
	CreatedAt        time.Time `json:"created_at"`
}

func (source *AuthSource) Normalize() {
	source.Type = strings.ToLower(source.Type)
	utils.TrimStringFields(
		&source.Name,
		&source.Type,
		&source.DisplayName,
		&source.ClientID,
		&source.ClientSecret,
		&source.OpenIDDiscoveryURL,
		&source.Scopes,
		&source.IconURL,
	)
	if source.DisplayName == "" {
		source.DisplayName = source.Name
	}
	if source.Type == AuthSourceTypeOIDC && source.Scopes == "" {
		source.Scopes = "openid profile email"
	}
	if source.Type == AuthSourceTypeGitHub && source.Scopes == "" {
		source.Scopes = "user:email"
	}
}

func (source *AuthSource) Validate() error {
	source.Normalize()
	if source.Name == "" {
		return errors.New("认证源名称不能为空")
	}
	if !authSourceNamePattern.MatchString(source.Name) {
		return errors.New("认证源名称只能包含字母、数字、短横线或下划线，且必须以字母或数字开头")
	}
	switch source.Type {
	case AuthSourceTypeGitHub:
	case AuthSourceTypeOIDC:
		if source.OpenIDDiscoveryURL == "" {
			return errors.New("OIDC 认证源必须配置 Discovery URL")
		}
	default:
		return errors.New("认证源类型仅支持 github 或 oidc")
	}
	if source.IsActive {
		if source.ClientID == "" || source.ClientSecret == "" {
			return errors.New("启用认证源前必须配置 Client ID 和 Client Secret")
		}
	}
	return nil
}

func (source *AuthSource) Sanitize() {
	source.ClientSecretConfigured = source.ClientSecret != ""
	source.ClientSecret = ""
}

func GetAuthSources() ([]AuthSource, error) {
	var sources []AuthSource
	err := DB.Order("id asc").Find(&sources).Error
	for index := range sources {
		sources[index].Sanitize()
	}
	return sources, err
}

func GetActiveAuthSources() ([]AuthSource, error) {
	var sources []AuthSource
	err := DB.Where("is_active = ?", true).Order("id asc").Find(&sources).Error
	for index := range sources {
		sources[index].Sanitize()
	}
	return sources, err
}

func GetAuthSourceByID(id uint) (*AuthSource, error) {
	if id == 0 {
		return nil, errors.New("认证源 ID 不能为空")
	}
	var source AuthSource
	if err := DB.First(&source, "id = ?", id).Error; err != nil {
		return nil, err
	}
	source.ClientSecretConfigured = source.ClientSecret != ""
	return &source, nil
}

func GetAuthSourceByName(name string) (*AuthSource, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("认证源名称不能为空")
	}
	var source AuthSource
	if err := DB.First(&source, "name = ?", name).Error; err != nil {
		return nil, err
	}
	source.ClientSecretConfigured = source.ClientSecret != ""
	return &source, nil
}

func CreateAuthSource(source *AuthSource) error {
	if err := source.Validate(); err != nil {
		return err
	}
	return DB.Create(source).Error
}

func UpdateAuthSource(source *AuthSource, keepSecret bool) error {
	if source.ID == 0 {
		return errors.New("认证源 ID 不能为空")
	}
	var current AuthSource
	if err := DB.First(&current, "id = ?", source.ID).Error; err != nil {
		return err
	}
	if keepSecret {
		source.ClientSecret = current.ClientSecret
	}
	if err := source.Validate(); err != nil {
		return err
	}
	return DB.Model(&current).Updates(map[string]any{
		"name":                 source.Name,
		"type":                 source.Type,
		"display_name":         source.DisplayName,
		"is_active":            source.IsActive,
		"client_id":            source.ClientID,
		"client_secret":        source.ClientSecret,
		"openid_discovery_url": source.OpenIDDiscoveryURL,
		"scopes":               source.Scopes,
		"icon_url":             source.IconURL,
	}).Error
}

func ToggleAuthSource(id uint, isActive bool) error {
	source, err := GetAuthSourceByID(id)
	if err != nil {
		return err
	}
	source.IsActive = isActive
	if err := source.Validate(); err != nil {
		return err
	}
	return DB.Model(&AuthSource{}).Where("id = ?", id).Update("is_active", isActive).Error
}

func DeleteAuthSource(id uint) error {
	if id == 0 {
		return errors.New("认证源 ID 不能为空")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("auth_source_id = ?", id).Delete(&ExternalAccount{}).Error; err != nil {
			return err
		}
		return tx.Delete(&AuthSource{}, "id = ?", id).Error
	})
}

func FindExternalAccount(sourceID uint, externalID string) (*ExternalAccount, error) {
	var account ExternalAccount
	err := DB.Where("auth_source_id = ? AND external_id = ?", sourceID, externalID).First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func LinkExternalAccount(account *ExternalAccount) error {
	if account.AuthSourceID == 0 || account.UserID == 0 || strings.TrimSpace(account.ExternalID) == "" {
		return errors.New("外部账号绑定信息不完整")
	}
	account.ExternalID = strings.TrimSpace(account.ExternalID)
	account.ExternalUsername = strings.TrimSpace(account.ExternalUsername)
	account.Email = strings.TrimSpace(account.Email)
	return DB.Where(ExternalAccount{
		AuthSourceID: account.AuthSourceID,
		ExternalID:   account.ExternalID,
	}).FirstOrCreate(account).Error
}

func ListExternalAccountsByUserID(userID int) ([]ExternalAccountView, error) {
	if userID <= 0 {
		return nil, errors.New("用户 ID 不能为空")
	}
	var accounts []ExternalAccount
	if err := DB.Preload("AuthSource").Where("user_id = ?", userID).Order("id asc").Find(&accounts).Error; err != nil {
		return nil, err
	}
	views := make([]ExternalAccountView, 0, len(accounts))
	for _, account := range accounts {
		label := account.AuthSource.DisplayName
		if label == "" {
			label = account.AuthSource.Name
		}
		views = append(views, ExternalAccountView{
			ID:               account.ID,
			AuthSourceID:     account.AuthSourceID,
			AuthSourceName:   account.AuthSource.Name,
			AuthSourceType:   account.AuthSource.Type,
			AuthSourceLabel:  label,
			ExternalUsername: account.ExternalUsername,
			Email:            account.Email,
			CreatedAt:        account.CreatedAt,
		})
	}
	return views, nil
}

func DeleteExternalAccountForUser(id uint, userID int) error {
	if id == 0 {
		return errors.New("绑定记录 ID 不能为空")
	}
	if userID <= 0 {
		return errors.New("用户 ID 不能为空")
	}
	result := DB.Where("id = ? AND user_id = ?", id, userID).Delete(&ExternalAccount{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("绑定记录不存在")
	}
	return nil
}
