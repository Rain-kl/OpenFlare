package model

import "time"

type AcmeAccount struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Email      string    `json:"email" gorm:"size:255"`
	URL        string    `json:"url" gorm:"size:255"`
	PrivateKey string    `json:"-" gorm:"type:text;not null"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func GetAcmeAccountByID(id uint) (*AcmeAccount, error) {
	account := &AcmeAccount{}
	err := DB.First(account, id).Error
	return account, err
}

func GetDefaultAcmeAccount() (*AcmeAccount, error) {
	account := &AcmeAccount{}
	err := DB.Order("id asc").First(account).Error
	if err != nil {
		// Auto-create a default account placeholder if none exists
		account.Email = "admin@openflare.dev"
		err = DB.Create(account).Error
	}
	return account, err
}

func (account *AcmeAccount) Insert() error {
	return DB.Create(account).Error
}

func (account *AcmeAccount) Update() error {
	return DB.Save(account).Error
}

func (account *AcmeAccount) Delete() error {
	return DB.Delete(account).Error
}
