package models

type Association struct {
	TargetID int64     `json:"targetID" gorm:"primaryKey;type:text"`
	Target   RecordKey `json:"-" gorm:"foreignKey:TargetID;references:ID;constraint:OnDelete:CASCADE;"`
	ItemID   string    `json:"itemID" gorm:"primaryKey;type:text"`
	Item     Record    `json:"-" gorm:"foreignKey:ItemID;references:DocumentID;constraint:OnDelete:CASCADE;"`
	Unique   string    `json:"unique" gorm:"type:text;uniqueIndex:uniq_association"`
}
