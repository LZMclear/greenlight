package data

import (
	"Greenlight/internal/validator"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"time"
)

type User struct {
	ID           int       `json:"ID"`
	CreatedAt    time.Time `json:"created_at"`
	Username     string    `json:"username"`
	PasswordHash password  `json:"-"`
	Activated    bool      `json:"activated"`
	Version      int       `json:"version"`
	Email        string    `json:"email"`
}

type password struct {
	plaintext *string //纯文本格式
	hash      []byte
}

var (
	ErrDuplicateEmail = errors.New("duplicate email")
)

// AnonymousUser 声明一个匿名用户
var AnonymousUser = &User{}

type UserModelInterface interface {
	Insert(user *User) error
	GetByEmail(email string) (*User, error)
	Update(user *User) error
	GetForToken(scopeActivation, tokenPlaintext string) (*User, error)
	Get(id int64) (*User, error)
}

type UserModel struct {
	DB *sql.DB
}

// Insert 在这里是UserModel的指针类型实现了UsrModelInterface，所以需要使用指针类型为其赋值
// 感觉不太对劲，操作不太完美
func (m *UserModel) Insert(user *User) error {
	query := "insert into users (created_at,username,password_hash,activated,email)values (UTC_TIMESTAMP(),?, ?, ?, ?)"
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()
	_, err := m.DB.ExecContext(ctx, query, user.Username, user.PasswordHash.hash, user.Activated, user.Email)
	//有问题！！！当出现email冲突错误时，id会正常自增。
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "Duplicate entry"):
			return ErrDuplicateEmail
		default:
			return err
		}
	}
	//将数据取出来
	out, err := m.GetByEmail(user.Email)
	if err != nil {
		return err
	}
	user.ID = out.ID
	user.CreatedAt = out.CreatedAt
	user.Version = out.Version
	return nil
}

func (m *UserModel) GetByEmail(email string) (*User, error) {
	query := "select * from users where email =?"
	var user User
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()
	err := m.DB.QueryRowContext(ctx, query, email).Scan(&user.ID, &user.CreatedAt, &user.Username, &user.PasswordHash.hash, &user.Activated, &user.Version, &user.Email)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &user, nil
}

// Update 更新数据需要先取出来然后再更新
func (m *UserModel) Update(user *User) error {
	query := "update users set username = ?,password_hash = ?,activated = ?,version = version+1,email =? where id=? and version=?"
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()

	_, err := m.DB.ExecContext(ctx, query, user.Username, user.PasswordHash.hash, user.Activated, user.Email, user.ID, user.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrRecordNotFound
		default:
			return err
		}
	}
	return nil
}

func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email")
}

func ValidateUser(v *validator.Validator, u *User) {
	v.Check(u.Username != "", "username", "must be provided")
	v.Check(len(u.Username) < 500, "username", "must not be more than 500")
	ValidateEmail(v, u.Email)
	v.Check(*u.PasswordHash.plaintext != "", "password", "must be provided")
	v.Check(len(*u.PasswordHash.plaintext) >= 8, "password", "must be at least 8 bytes long")
	//在创建hash时，输入被截断为72字节。如果用户输入过长，在此之后的字段将被忽略，为了避免混淆，强制将密码长度限制在72字节
	v.Check(len(*u.PasswordHash.plaintext) <= 72, "password", "must not be more than 72 bytes long")
	if u.PasswordHash.hash == nil {
		panic("missing password hash for user")
	}
}

// Set 计算传递的密码hash值
func (p *password) Set(plainTextPassword string) error {
	//代价参数，成本越高，计算速度慢，需要在计算速度与安全性取折中
	bytes, err := bcrypt.GenerateFromPassword([]byte(plainTextPassword), 12)
	if err != nil {
		return err
	}
	p.plaintext = &plainTextPassword
	p.hash = bytes
	return nil
}

// Matches 检查传递的字符串类型的密码是否与结构体中的hash匹配
func (p *password) Matches(plainTextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plainTextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}
	return true, err
}

func (m *UserModel) GetForToken(scopeActivation, tokenPlaintext string) (*User, error) {
	query := "select users.id,users.created_at,users.username,users.password_hash,users.activated,users.version,users.email from users inner join tokens on users.id = tokens.user_id where tokens.hash_byte=? and tokens.scope = ? and tokens.expiry > ?"
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()
	//将数据进行hash处理
	bytes := sha256.Sum256([]byte(tokenPlaintext))
	//返回的是一个大小为32的byte数组，将其转换为切片
	hash := bytes[:]
	var user User
	err := m.DB.QueryRowContext(ctx, query, hash, scopeActivation, time.Now()).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Username,
		&user.PasswordHash.hash,
		&user.Activated,
		&user.Version,
		&user.Email,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &user, nil
}

// IsAnonymousUser 检查用户实例是否是匿名用户
func (u *User) IsAnonymousUser() bool {
	return u == AnonymousUser
}

// Get 根据ID获取用户数据
func (m *UserModel) Get(id int64) (*User, error) {
	query := "select * from users where id = ?"
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()
	var user User
	err := m.DB.QueryRowContext(ctx, query, id).Scan(&user.ID, &user.CreatedAt, &user.Username, &user.PasswordHash.hash, &user.Activated, &user.Version, &user.Email)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &user, nil
}
