package data

import (
	"Greenlight/internal/validator"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"time"
)

// ScopeActivation 定义token的类型，后面还会有其他的token类型，使用字段scope加以区分同一个数据表中的token
const (
	ScopeActivation     = "activation"
	ScopeAuthentication = "authentication"
	ScopePasswordReset  = "password-reset"
)

type Token struct {
	PlainText string    `json:"token"`
	Hash      []byte    `json:"_"`
	UserID    int       `json:"_"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"_"`
}

func generateToken(userID int, ttl time.Duration, scope string) (*Token, error) {
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl), //过期时间为当前时间加上存活时间
		Scope:  scope,
	}
	randomBytes := make([]byte, 16)
	//使用随机字节填充字节数组
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}
	//字节数组就是二进制数据，转换为base-32-encode的字符串方便人们阅读...
	/*
		明文令牌字符串本身的长度取决于如何对这16个随机字节进行编码以创建字符串。
		在本例中，我们将随机字节编码为基数为32的字符串，从而得到一个包含26个字符的字符串。
		相反，如果我们使用十六进制（基数为16）对随机字节进行编码，则字符串将变成32个字符长。
	*/
	//将字节编码为base-32-encode字符串并将他分配给token的Plaintext字段，这个是用来发送给用户的  向用户发送的是未经过hash处理的
	//注意，默认情况下base-32字符串会使用"="字符进行填充，我们不需要填充，所以使用WithPadding(base32.NoPadding) method
	token.PlainText = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	//生成明文令牌字符串的SHA-256散列
	bytes := sha256.Sum256([]byte(token.PlainText))
	//返回的是一个长度为32的字节数组，将他转变为一个字节切片赋值给Hash字段
	token.Hash = bytes[:]
	return token, nil
}

func ValidateTokenPlainText(v *validator.Validator, tokenPlaintext string) {
	v.Check(tokenPlaintext != "", "token", "must be provided")
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}

type TokenModelInterface interface {
	New(userID int, ttl time.Duration, scope string) (*Token, error)
	DeleteAllForUser(userID int, scope string) error
}

type TokenModel struct {
	DB *sql.DB
}

// New 创建一个token结构体并将其存入数据库中
func (m *TokenModel) New(userID int, ttl time.Duration, scope string) (*Token, error) {
	token, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}
	err = m.Insert(token)
	return token, err
}

/*
	存储时遇到的问题：
	1. 存储token.Hash类型时，由于Token.Hash是字节数组，使用mysql中的binary类型进行存储。
	它可以原封不动的存储字节序列。不会进行字符集编码转换。
	2.若使用char varchar类型，需要将字节数组转换为字符串。会涉及字符集编码的问题，可能会出现字符集编码不兼容导致数据损坏或错误。
	3. 具体的流程：
		1.首先按照默认的字符编码规则将字节数组转换为字符串
		2.MySQL 会根据该字段设置的字符集对字符串进行进一步的编码转换。如果 MySQL 字段的字符集与 Go 语言中使用的默认字符编码不一致，就会发生字符集转换。
		3.字符集转换过程中，如果字节数组中的某些字节序列在目标字符集中无法正确表示，就可能导致数据丢失或转换错误
*/

func (m *TokenModel) Insert(token *Token) error {
	query := "insert into tokens (hash_byte,user_id,expiry,scope) values (?,?,?,?)"
	//创建一个超时的上下文
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()
	_, err := m.DB.ExecContext(ctx, query, token.Hash, token.UserID, token.Expiry, token.Scope)
	return err
}

func (m *TokenModel) DeleteAllForUser(userID int, scope string) error {
	query := "delete from tokens where user_id=? and scope=?"
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()
	_, err := m.DB.ExecContext(ctx, query, userID, scope)
	return err
}
