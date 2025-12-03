package models

import (
	"github.com/lib/pq"
	"time"
)

type CommitOwner struct {
	CommitLogID string    `json:"commit_log_id" gorm:"type:text;primaryKey"`
	CommitLog   CommitLog `json:"-" gorm:"constraint:OnDelete:CASCADE;"`
	Owner       string    `json:"owner" gorm:"type:text;index;primaryKey"`
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
	ID       int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Owner    string `json:"owner" gorm:"uniqueIndex:idx_owner_key;type:text"`
	Key      string `json:"key" gorm:"uniqueIndex:idx_owner_key;type:text"`
	RecordID string `json:"recordID" gorm:"type:text"`
	Record   Record `json:"record" gorm:"foreignKey:RecordID;references:DocumentID;constraint:OnDelete:CASCADE;"`
}

type Record struct {
	DocumentID  string         `json:"id" gorm:"primaryKey;type:text"`
	Document    CommitLog      `json:"-" gorm:"foreignKey:DocumentID;references:ID;constraint:OnDelete:CASCADE;"`
	ContentType string         `json:"contentType" gorm:"type:text"`
	Author      string         `json:"author" gorm:"type:text"`
	Schema      string         `json:"schema" gorm:"type:text"`
	Value       string         `json:"value" gorm:"type:jsonb"`
	Reference   string         `json:"reference" gorm:"type:text"`
	Referenced  pq.StringArray `json:"referenced" gorm:"type:text[]"`
	CDate       time.Time      `json:"cdate" gorm:"->;<-:create;type:timestamp with time zone;not null;default:clock_timestamp()"`
}

type CollectionMember struct {
	CollectionID int64     `json:"collectionID" gorm:"primaryKey;type:text"`
	Collection   RecordKey `json:"-" gorm:"foreignKey:CollectionID;references:ID;constraint:OnDelete:CASCADE;"`
	ItemID       int64     `json:"itemID" gorm:"primaryKey;type:text"`
	Item         RecordKey `json:"-" gorm:"foreignKey:ItemID;references:ID;constraint:OnDelete:CASCADE;"`
}
