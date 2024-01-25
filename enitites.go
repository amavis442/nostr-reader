package main

import (
	"database/sql/driver"
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type EventTable struct {
	gorm.Model
	EventId        string `gorm:"not null; unique; index"`
	Pubkey         string `gorm:"index;not null"`
	Kind           int    `gorm:"not null"`
	EventCreatedAt int64  `gorm:"not null"`
	Content        string
	TagsFull       string
	Ptags          pq.StringArray `gorm:"type:text[];index:idx_event_tables_ptags,type:gin"`
	Etags          pq.StringArray `gorm:"type:text[];index:idx_event_tables_etags,type:gin"`
	Sig            string         `gorm:"not null"`
	Garbage        bool           `gorm:"default:false"`
	Raw            datatypes.JSON
	VoteTable      []VoteTable `gorm:"foreignKey:TargetEventId;references:EventId;default:null"`
}

type ProfileTable struct {
	gorm.Model
	Pubkey      string `gorm:"index;not null;unique"`
	Name        string
	About       string
	Picture     string
	Website     string
	Nip05       string
	Lud16       string
	DisplayName string
	Raw         string
	EventTable  []EventTable `gorm:"foreignKey:Pubkey;references:Pubkey;default:null"`
}

type BlockPubkeyTable struct {
	ID        uint
	Pubkey    string    `gorm:"index;not null;unique"`
	CreatedAt time.Time `gorm:"default:current_timestamp"`
}

type FollowPubkeyTable struct {
	ID        uint
	Pubkey    string    `gorm:"index;not null;unique"`
	CreatedAt time.Time `gorm:"default:current_timestamp"`
}

type SeenTable struct {
	gorm.Model
	EventId string `gorm:"index;not null;unique"`
}

type TreeTable struct {
	ID           uint
	EventId      string    `gorm:"index;not null;unique"`
	RootEventId  string    `gorm:"index;not null;"`
	ReplyEventId string    `gorm:"index;not null;"`
	CreatedAt    time.Time `gorm:"default:current_timestamp"`
}

type BookmarkTable struct {
	ID           uint
	EventId      string `gorm:"index;not null;unique"`
	EventTableID uint
	CreatedAt    time.Time `gorm:"default:current_timestamp"`
}

type Vote string

const (
	Like    Vote = "like"
	Dislike Vote = "dislike"
)

func (p *Vote) Scan(value interface{}) error {
	*p = Vote(value.([]byte))
	return nil
}

func (p Vote) Value() (driver.Value, error) {
	return string(p), nil
}

type VoteTable struct {
	gorm.Model
	Pubkey        string `gorm:"index:idx_vote_tables_pubkey_target;unique;not null"`
	Content       string `gorm:"not null"`
	CurrentVote   Vote   `gorm:"type:vote;not null"`
	TargetEventId string `gorm:"index:idx_vote_tables_pubkey_target;unique;not null"`
	FromEventId   string `gorm:"index;not null"`
	EventTableID  uint
}
