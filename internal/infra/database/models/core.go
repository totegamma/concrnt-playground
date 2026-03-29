package models

import (
	"time"

	"github.com/lib/pq"
)

type CommitOwner struct {
	CommitLogID string    `json:"commit_log_id" gorm:"type:text;primaryKey"`
	CommitLog   CommitLog `json:"-" gorm:"constraint:OnDelete:CASCADE;"`
	Owner       string    `json:"owner" gorm:"type:text;primaryKey"`
	CDate       time.Time `json:"cdate" gorm:"->;<-:create;type:timestamp with time zone;not null;default:clock_timestamp()"`
}

type CommitLog struct {
	ID          string    `json:"id" gorm:"primaryKey;type:text"`
	Document    string    `json:"document" gorm:"type:text"`
	Proof       string    `json:"proof" gorm:"type:text"`
	GcCandidate bool      `json:"gcCandidate" gorm:"type:boolean;not null;default:false;index"`
	CDate       time.Time `json:"cdate" gorm:"type:timestamp with time zone;not null;default:clock_timestamp()"`
}

type RecordKey struct {
	ID       int64   `json:"id" gorm:"primaryKey;autoIncrement"`
	ParentID *int64  `json:"parentID" gorm:"index"`
	URI      string  `json:"uri" gorm:"type:text;unique"`
	RecordID *string `json:"recordID" gorm:"type:text"`
	Record   Record  `json:"record" gorm:"foreignKey:RecordID;references:DocumentID;constraint:OnDelete:CASCADE;"`
}

type Record struct {
	DocumentID    string         `json:"id" gorm:"primaryKey;type:text"`
	Document      CommitLog      `json:"documnet" gorm:"foreignKey:DocumentID;references:ID;constraint:OnDelete:CASCADE;"`
	Owner         string         `json:"owner" gorm:"type:text"`
	Schema        string         `json:"schema" gorm:"type:text"`
	Policies      *string        `json:"policies" gorm:"type:text"`
	Distributions pq.StringArray `json:"distributions" gorm:"type:text[]"`
	CDate         time.Time      `json:"cdate" gorm:"->;<-:create;type:timestamp with time zone;not null;default:clock_timestamp()"`
}

type Ack struct {
	From    string `json:"from" gorm:"type:text;index"`
	To      string `json:"to" gorm:"type:text;index"`
	Context string `json:"schema" gorm:"type:text;index"`

	DocumentID string    `json:"id" gorm:"primaryKey;type:text"`
	Document   CommitLog `json:"-" gorm:"foreignKey:DocumentID;references:ID;constraint:OnDelete:CASCADE;"`

	Valid bool `json:"valid" gorm:"type:boolean;not null;default:true"`

	CDate time.Time `json:"cdate" gorm:"->;<-:create;type:timestamp with time zone;not null;default:clock_timestamp()"`
}
