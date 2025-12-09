package models

import (
	"time"
)

type Entity struct {
	ID                   string    `json:"ccid" gorm:"type:text"`
	Alias                *string   `json:"alias,omitempty" gorm:"type:text"`
	Domain               string    `json:"domain" gorm:"type:text"`
	Tag                  string    `json:"tag" gorm:"type:text;"`
	AffiliationDocument  string    `json:"affiliationDocument" gorm:"type:text"`
	AffiliationSignature string    `json:"affiliationSignature" gorm:"type:text"`
	CDate                time.Time `json:"cdate" gorm:"->;<-:create;type:timestamp with time zone;not null;default:clock_timestamp()"`
	MDate                time.Time `json:"mdate" gorm:"autoUpdateTime"`
}

type EntityMeta struct {
	ID      string  `json:"ccid" gorm:"type:text"`
	Inviter *string `json:"inviter" gorm:"type:text"`
	Info    string  `json:"info" gorm:"type:jsonb;default:'null'"`
}
