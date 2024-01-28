package database

import (
	"database/sql/driver"
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Note struct {
	ID             uint
	EventId        string `gorm:"not null; unique; index;type:varchar(100)"`
	Pubkey         string `gorm:"index,type:btree;not null;type:varchar(100)"`
	Kind           int    `gorm:"not null"`
	EventCreatedAt int64  `gorm:"not null"`
	Content        string
	TagsFull       string
	Ptags          pq.StringArray `gorm:"type:text[];index:idx_notes_ptags,type:gin"`
	Etags          pq.StringArray `gorm:"type:text[];index:idx_notes_etags,type:gin"`
	Sig            string         `gorm:"not null;type:varchar(200)"`
	Garbage        bool           `gorm:"default:false"`
	Raw            datatypes.JSON
	Reaction       []Reaction `gorm:"default:null"`
	ProfileID      uint       `gorm:"default:null"`
	FollowID       uint       `gorm:"default:null"`
	BlockID        uint       `gorm:"default:null"`
	CreatedAt      time.Time  `gorm:"default:current_timestamp"`
	UpdatedAt      time.Time  `gorm:"default:null"`
}

func (entity *Note) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Profile struct {
	ID          uint
	Pubkey      string `gorm:"index,type:btree;not null;unique;type:varchar(100)"`
	Name        string
	About       string
	Picture     string
	Website     string
	Nip05       string
	Lud16       string
	DisplayName string
	Raw         string
	EventTable  []Note    `gorm:"default:null"` //`gorm:"foreignKey:Pubkey;references:Pubkey;default:null"`
	CreatedAt   time.Time `gorm:"default:current_timestamp"`
	UpdatedAt   time.Time `gorm:"default:null"`
}

func (entity *Profile) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Block struct {
	ID        uint
	Pubkey    string    `gorm:"index,type:btree;not null;unique;type:varchar(100)"`
	Note      []Note    `gorm:"default:null"`
	CreatedAt time.Time `gorm:"default:current_timestamp"`
	UpdatedAt time.Time `gorm:"default:null"`
}

func (entity *Block) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Follow struct {
	ID         uint
	Pubkey     string    `gorm:"index,type:btree;unique;type:varchar(100)"`
	EventTable []Note    `gorm:"default:null"`
	CreatedAt  time.Time `gorm:"default:current_timestamp"`
	UpdatedAt  time.Time `gorm:"default:null"`
}

func (entity *Follow) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Seen struct {
	ID        uint
	EventId   string `gorm:"index,type:btree;not null;unique;type:varchar(100)"`
	NoteID    uint
	CreatedAt time.Time `gorm:"default:current_timestamp"`
	UpdatedAt time.Time `gorm:"default:null"`
}

func (entity *Seen) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Tree struct {
	ID           uint
	EventId      string    `gorm:"index,type:btree;not null;unique;type:varchar(100)"`
	RootEventId  string    `gorm:"index,type:btree;not null;type:varchar(100)"`
	ReplyEventId string    `gorm:"index,type:btree;not null;type:varchar(100)"`
	CreatedAt    time.Time `gorm:"default:current_timestamp"`
	UpdatedAt    time.Time `gorm:"default:null"`
}

func (entity *Tree) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Bookmark struct {
	ID        uint
	EventId   string `gorm:"index;not null;unique;type:varchar(100)"`
	NoteID    uint
	CreatedAt time.Time `gorm:"default:current_timestamp"`
	UpdatedAt time.Time `gorm:"default:null"`
}

func (entity *Bookmark) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
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

type Reaction struct {
	ID            uint
	Pubkey        string    `gorm:"index:idx_vote_tables_pubkey_target;unique;not null"`
	Content       string    `gorm:"not null"`
	CurrentVote   Vote      `gorm:"type:vote;not null"`
	TargetEventId string    `gorm:"index:idx_vote_tables_pubkey_target;unique;not null"`
	FromEventId   string    `gorm:"index;not null"`
	NoteID        uint      `gorm:"default:null"`
	CreatedAt     time.Time `gorm:"default:current_timestamp"`
	UpdatedAt     time.Time `gorm:"default:null"`
}

func (entity *Reaction) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}
