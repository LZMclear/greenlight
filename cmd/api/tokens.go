package main

import (
	"Greenlight/internal/data"
	"Greenlight/internal/validator"
	"net/http"
)

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
}
