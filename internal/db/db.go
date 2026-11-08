package db

import (
	"fmt"
	"nsf/model"
)

var (
	NSF_DB_PATH = "/tmp/nsf.db"
)

type DbIf interface {
	Release() error

	Save(key, value string) error
	SaveAll(data map[string]string) error
	Exist(key string) (bool, error)
	Load(key string) (string, error)
	LoadAll() (map[string]string, error)
	Remove(key string) error
	RemoveAll() error
}

func NewDb(dbType string, mongodb ...model.MongoDBIE) (DbIf, error) {
	switch dbType {
	case "bbolt":
		return newBboltDb(NSF_DB_PATH)
	// TODO: Add support for the customDb type here
	case "custom":
		var config model.MongoDBIE
		if len(mongodb) > 0 {
			config = mongodb[0]
		}
		return newCustomDb(config.URI, config.Name)
	}
	return nil, fmt.Errorf("unsupported db type: %s", dbType)
}
