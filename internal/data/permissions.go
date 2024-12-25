package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PermissionModelInterface 创建一个PermissionModelInterface接口
type PermissionModelInterface interface {
	GetAllForUser(id int) (Permissions, error)
	AddForUser(userID int, codes ...string) error
}

// PermissionModel Permissions 创建一个permissions结构体实现接口
type PermissionModel struct {
	DB *sql.DB
}

type Permissions []string

//添加一个帮助方法检查Permissions是否包含一个特定的permission code

func (p Permissions) Include(code string) bool {
	for i := range p {
		if p[i] == code {
			return true
		}
	}
	return false
}

// GetAllForUser 根据用户id获取该用户的全部权限
func (m *PermissionModel) GetAllForUser(id int) (Permissions, error) {
	query := "select permissions.code from permissions inner join users_permissions on permissions.id = users_permissions.permission_id inner join users on users.id = users_permissions.user_id where users.id=?"
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()
	rows, err := m.DB.QueryContext(ctx, query, id)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	var permissions Permissions
	for rows.Next() {
		var permission string
		err := rows.Scan(&permission)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return permissions, nil
}

func (m *PermissionModel) AddForUser(userID int, codes ...string) error {
	//构建in子句和占位符
	placeholders := strings.Repeat("?,", len(codes)-1) + "?"
	inClause := fmt.Sprintf("IN (%s)", placeholders)
	//构建查询语句
	query := fmt.Sprintf("insert into users_permissions select ?,permissions.id from permissions where permissions.code %s", inClause)

	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()
	if len(codes) == 2 {
		_, err := m.DB.ExecContext(ctx, query, userID, codes[0], codes[1])
		if err != nil {
			return err
		}
	} else {
		_, err := m.DB.ExecContext(ctx, query, userID, codes[0])
		if err != nil {
			return err
		}
	}
	return nil
}
