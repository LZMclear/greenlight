package main

import (
	"Greenlight/internal/data"
	"Greenlight/internal/validator"
	"errors"
	"github.com/pascaldekloe/jwt"
	"net/http"
	"strconv"
	"time"
)

// 创建有状态的token，有状态就是将创建的token存入数据库中，当用户使用完后删除
func (app *application) createAuthenticationTokenHandler(w http.ResponseWriter, r *http.Request) {
	//从请求体中解析邮件和密码
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	//验证数据格式的准确性
	v := validator.New()
	data.ValidateEmail(v, input.Email)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	//根据email取出用户信息
	user, err := app.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	//查看客户端发送的密码是否与数据库中的匹配
	match, err := user.PasswordHash.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}
	token, err := app.models.Tokens.New(user.ID, 24*time.Hour, data.ScopeAuthentication)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	err = app.writeJSON(w, http.StatusCreated, nil, envelope{"authentication_token": token})
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// 接收用户email和password，创建授权token返回
func (app *application) createAuthenticationJWTTokenHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	//验证数据合法性
	v := validator.New()
	data.ValidateEmail(v, input.Email)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	//检查数据库中是否含有此用户
	user, err := app.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	matches, err := user.PasswordHash.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	if !matches {
		app.invalidCredentialsResponse(w, r)
		return
	}
	//创建一个JWT生命结构，其中包含用户ID为主题，发布时间为现在，有效窗口期为接下来的24小时。还将发行人和受众设置为应用程序的唯一标识
	var claim jwt.Claims
	claim.Subject = strconv.FormatInt(int64(user.ID), 10)              //用户ID为主题
	claim.Issued = jwt.NewNumericTime(time.Now())                      //发布时间
	claim.NotBefore = jwt.NewNumericTime(time.Now())                   //生效时间
	claim.Expires = jwt.NewNumericTime(time.Now().Add(time.Hour * 24)) //有效期
	claim.Issuer = "greenlight.alexedwards.net"                        //发行人，通常是一个可以信任的服务器或系统
	claim.Audiences = []string{"greenlight.alexedwards.net"}           //受众是jwt的接收者，指定了jwt的合法接收方
	//使用HMAC-SHA256算法和应用程序中的密钥对jwt声明签名，它返回一个字节切片，包含以base64编码的字符串形式存在的JWT
	sign, err := claim.HMACSign(jwt.HS256, []byte(app.config.jwt.secret))
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	//返回给客户端
	err = app.writeJSON(w, http.StatusOK, nil, envelope{"authentication_token": string(sign)})
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) createPasswordResetTokenHandler(w http.ResponseWriter, r *http.Request) {
	//定义匿名结构体解析邮箱地址
	var input struct {
		Email string `json:"email"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	//验证邮箱的合法性
	v := validator.New()
	data.ValidateEmail(v, input.Email)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	user, err := app.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("email", "not found email address")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	if !user.Activated {
		v.AddError("activate", "the user does not activate")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	token, err := app.models.Tokens.New(user.ID, 45*time.Minute, data.ScopePasswordReset)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	//开启一个线程用来发送token
	app.background(func() {
		data := map[string]interface{}{
			"passwordResetToken": token.PlainText,
		}
		err = app.mailer.Send(user.Email, "token_password_reset.html", data)
		if err != nil {
			app.logger.PrintError(err, nil)
		}
	})
	//向客户端发送一个202和确认消息
	err = app.writeJSON(w, http.StatusAccepted, nil, envelope{"message": "an email will be sent to you containing password reset instructions"})
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// 验证密码重置令牌，为用户设置新密码
func (app *application) updateUserPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Password       string `json:"password"`
		TokenPlainText string `json:"token"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	//验证数据合法性
	v := validator.New()
	data.ValidateTokenPlainText(v, input.TokenPlainText)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	user, err := app.models.Users.GetForToken(data.ScopePasswordReset, input.TokenPlainText)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired password reset token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	err = user.PasswordHash.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	err = app.models.Users.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	//删除用户所有的token
	err = app.models.Tokens.DeleteAllForUser(user.ID, data.ScopePasswordReset)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	err = app.writeJSON(w, http.StatusOK, nil, envelope{"message": "your password has reset successfully"})
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) createActivateTokenHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	//验证数据合法性
	v := validator.New()
	data.ValidateEmail(v, input.Email)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	//查询当前数据库是否有这个邮箱的用户
	user, err := app.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("email", "no matching email address found")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	//用户存在，检查用户是否激活
	if user.Activated {
		v.AddError("activate", "the user has already activate")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	//生成激活token使用邮箱发送
	token, err := app.models.Tokens.New(user.ID, 3*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	app.background(func() {
		err = app.mailer.Send(user.Email, "token_activation.html", map[string]interface{}{
			"activationToken": token.PlainText,
		})
		if err != nil {
			app.logger.PrintError(err, nil)
		}
	})
	err = app.writeJSON(w, http.StatusOK, nil, envelope{"message": "an email will be sent to you containing activation instruction"})
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
