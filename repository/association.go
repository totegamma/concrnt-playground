package repository

type Association struct {
	TargetID string `json:"targetID" gorm:"primaryKey;type:text"`
	Target   Record `json:"-" gorm:"foreignKey:TargetID;references:DocumentID;constraint:OnDelete:CASCADE;"`
	ItemID   string `json:"itemID" gorm:"primaryKey;type:text"`
	Item     Record `json:"-" gorm:"foreignKey:ItemID;references:DocumentID;constraint:OnDelete:CASCADE;"`
	Owner    string `json:"owner" gorm:"primaryKey;type:text"`
}
