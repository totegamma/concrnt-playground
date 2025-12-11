package models

type Association struct {
	TargetID int64     `json:"targetID" gorm:"primaryKey;type:text"`
	Target   RecordKey `json:"-" gorm:"foreignKey:TargetID;references:ID;constraint:OnDelete:CASCADE;"`
	ItemID   int64     `json:"itemID" gorm:"primaryKey;type:text"`
	Item     RecordKey `json:"-" gorm:"foreignKey:ItemID;references:ID;constraint:OnDelete:CASCADE;"`
	Unique   uint64    `json:"unique" gorm:"type:bigint;uniqueIndex:uniq_association"`
}
