package main

import (
	"Greenlight/internal/data"
	"Greenlight/internal/validator"
	"errors"
	"net/http"
	"time"
)

// bug，如果token创建失败，但是前面的用户会成功创建。
func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	//1. 创建一个匿名结构体用来接收请求体中的数据
	var input struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	//2. 解析请求体中的数据
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	var user data.User
	user.Email = input.Email
	user.Username = input.Username
	user.Activated = false
	user.PasswordHash.Set(input.Password)
	//3. 验证数据合法性
	v := validator.New()
	data.ValidateUser(v, &user)
	if !v.Valid() { //说明数据有错误
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	//4. 执行插入操作
	err = app.models.Users.Insert(&user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with email address already exist")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	//5. 成功插入数据后，生成一个激活令牌
	token, err := app.models.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	//5. 开启一个线程用于发送欢迎邮件，不会影响主程序的正常运行，减少响应的延迟。
	//  在这种后台线程中发生是的任何panic都不会被recoverPanic中间件或者http.Server恢复，
	app.background(func() {
		data := map[string]interface{}{
			"activationToken": token.PlainText,
			"userID":          user.ID,
		}
		err = app.mailer.Send(user.Email, "user_welcome.html", data)
		if err != nil {
			app.logger.PrintError(err, nil)
		}
	})

	err = app.writeJSON(w, http.StatusCreated, nil, envelope{"user": user})
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

}

func (app *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TokenPlaintext string `json:"token"`
	}
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	//校验令牌的合法性
	v := validator.New()
	data.ValidateTokenPlainText(v, input.TokenPlaintext)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	//提取与token相关的用户详细信息
	user, err := app.models.Users.GetForToken(data.ScopeActivation, input.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activate token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	//更新用户激活状态
	user.Activated = true
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
	//如果全部执行成功，用户状态为激活状态，删除与用户相关的所有token
	err = app.models.Tokens.DeleteAllForUser(user.ID, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	//返回更新后的用户
	err = app.writeJSON(w, http.StatusOK, nil, envelope{"user": user})
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
