package repository

import (
	"time"
)

// Entity is one of a concurrent base object
// mutable
type Entity struct {
	ID                   string    `json:"ccid" gorm:"type:char(42)"`
	Domain               string    `json:"domain" gorm:"type:text"`
	Tag                  string    `json:"tag" gorm:"type:text;"`
	Score                int       `json:"score" gorm:"type:integer;default:0"`
	IsScoreFixed         bool      `json:"isScoreFixed" gorm:"type:boolean;default:false"`
	AffiliationDocument  string    `json:"affiliationDocument" gorm:"type:json"`
	AffiliationSignature string    `json:"affiliationSignature" gorm:"type:char(130)"`
	TombstoneDocument    *string   `json:"tombstoneDocument" gorm:"type:json;default:null"`
	TombstoneSignature   *string   `json:"tombstoneSignature" gorm:"type:char(130);default:null"`
	Alias                *string   `json:"alias,omitempty" gorm:"type:text"`
	CDate                time.Time `json:"cdate" gorm:"->;<-:create;type:timestamp with time zone;not null;default:clock_timestamp()"`
	MDate                time.Time `json:"mdate" gorm:"autoUpdateTime"`
}

type EntityMeta struct {
	ID      string  `json:"ccid" gorm:"type:char(42)"`
	Inviter *string `json:"inviter" gorm:"type:char(42)"`
	Info    string  `json:"info" gorm:"type:json;default:'null'"`
}
