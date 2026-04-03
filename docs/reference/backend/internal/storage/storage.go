package storage

import (
	"encoding/json"
	"time"

	"tanabata/internal/domain"
)

type Storage interface {
	FileRepository
	Close()
}

type FileRepository interface {
	GetSlice(user_id int, filter, sort string, limit, offset int) (files domain.Slice[domain.FileItem], statusCode int, err error)
	Get(user_id int, file_id string) (file domain.FileFull, statusCode int, err error)
	Add(user_id int, name, mime string, datetime time.Time, notes string, metadata json.RawMessage) (file domain.FileCore, statusCode int, err error)
	Update(user_id int, file_id string, updates map[string]interface{}) (statusCode int, err error)
	Delete(user_id int, file_id string) (statusCode int, err error)
}
