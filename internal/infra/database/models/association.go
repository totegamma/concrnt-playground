package models

import (
	"time"
)

type Association struct {
	TargetID int64     `json:"targetID" gorm:"type:text"`
	Target   RecordKey `json:"-" gorm:"foreignKey:TargetID;references:ID;constraint:OnDelete:CASCADE;"`

	DocumentID string    `json:"id" gorm:"primaryKey;type:text"`
	Document   CommitLog `json:"-" gorm:"foreignKey:DocumentID;references:ID;constraint:OnDelete:CASCADE;"`

	Owner  string `json:"owner" gorm:"type:text"`
	Author string `json:"author" gorm:"type:text"`

	Schema  string  `json:"schema" gorm:"type:text"`
	Variant *string `json:"variant" gorm:"type:text"`
	Unique  string  `json:"unique" gorm:"type:text;unique"`

	CDate time.Time `json:"cdate" gorm:"->;<-:create;type:timestamp with time zone;not null;default:clock_timestamp()"`
}
