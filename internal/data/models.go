package data

import (
	"database/sql"
	"errors"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

// 将模型封装在父模型结构体中，提供了一个方便单一的容器，可以表示所有的数据库模型。

// Models 将会在里面添加其他的模型结构体，例如，UserModels, PermissionModels
type Models struct {
	Movies MovieModelInterface
	Users  UserModelInterface
	Tokens TokenModelInterface
}

func NewModels(db *sql.DB) Models {
	return Models{
		Movies: &MovieModel{db},
		Users:  &UserModel{db},
		Tokens: &TokenModel{DB: db},
	}
}
