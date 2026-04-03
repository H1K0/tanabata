package domain

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type User struct {
	Name      string `json:"name"`
	IsAdmin   bool   `json:"isAdmin"`
	CanCreate bool   `json:"canCreate"`
}

type MIME struct {
	Name      string `json:"name"`
	Extension string `json:"extension"`
}

type (
	CategoryCore struct {
		ID    string      `json:"id"`
		Name  string      `json:"name"`
		Color pgtype.Text `json:"color"`
	}
	CategoryItem struct {
		CategoryCore
	}
	CategoryFull struct {
		CategoryCore
		CreatedAt time.Time   `json:"createdAt"`
		Creator   User        `json:"creator"`
		Notes     pgtype.Text `json:"notes"`
	}
)

type (
	FileCore struct {
		ID   string      `json:"id"`
		Name pgtype.Text `json:"name"`
		MIME MIME        `json:"mime"`
	}
	FileItem struct {
		FileCore
		CreatedAt time.Time `json:"createdAt"`
		Creator   User      `json:"creator"`
	}
	FileFull struct {
		FileCore
		CreatedAt time.Time       `json:"createdAt"`
		Creator   User            `json:"creator"`
		Notes     pgtype.Text     `json:"notes"`
		Metadata  json.RawMessage `json:"metadata"`
		Tags      []TagCore       `json:"tags"`
		Viewed    int             `json:"viewed"`
	}
)

type (
	TagCore struct {
		ID    string      `json:"id"`
		Name  string      `json:"name"`
		Color pgtype.Text `json:"color"`
	}
	TagItem struct {
		TagCore
		Category CategoryCore `json:"category"`
	}
	TagFull struct {
		TagCore
		Category  CategoryCore `json:"category"`
		CreatedAt time.Time    `json:"createdAt"`
		Creator   User         `json:"creator"`
		Notes     pgtype.Text  `json:"notes"`
		UsedIncl  int          `json:"usedIncl"`
		UsedExcl  int          `json:"usedExcl"`
	}
)

type Autotag struct {
	TriggerTag TagCore `json:"triggerTag"`
	AddTag     TagCore `json:"addTag"`
	IsActive   bool    `json:"isActive"`
}

type (
	PoolCore struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	PoolItem struct {
		PoolCore
	}
	PoolFull struct {
		PoolCore
		CreatedAt time.Time   `json:"createdAt"`
		Creator   User        `json:"creator"`
		Notes     pgtype.Text `json:"notes"`
		Viewed    int         `json:"viewed"`
	}
)

type Session struct {
	ID           int       `json:"id"`
	UserAgent    string    `json:"userAgent"`
	StartedAt    time.Time `json:"startedAt"`
	ExpiresAt    time.Time `json:"expiresAt"`
	LastActivity time.Time `json:"lastActivity"`
}

type Pagination struct {
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Count  int `json:"count"`
}

type Slice[T any] struct {
	Pagination Pagination `json:"pagination"`
	Data       []T        `json:"data"`
}
