package main

import (
	"Greenlight/internal/data"
	"Greenlight/internal/validator"
	"errors"
	"fmt"
	"net/http"
)

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	//声明一个匿名结构体保存我们期望在请求中获取的数据
	var input struct {
		Title   string       `json:"title"`
		Year    int32        `json:"year"`
		Runtime data.Runtime `json:"runtime"` //使用自己定义的类型。
		Genres  []string     `json:"genres"`
	}
	//必须传入非空指针，否则会返回错误json.InvalidUnmarshalError
	//将 JSON 对象解码为结构时，JSON 中的键/值对会根据结构标签名称映射到结构字段（完全匹配是首选匹配项，但它将回退到不区分大小写的匹配项）。
	//任何无法成功映射到 struct 字段的 JSON 键/值对都将被静默忽略。
	//读取r.Body后，不需要关闭它，这由http.Server自动完成
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	//将input的值复制到movie中
	movie := &data.Movie{
		Title:   input.Title,
		Year:    input.Year,
		Runtime: input.Runtime,
		Genres:  input.Genres,
	}
	//初始化一个新的校验器实例
	v := validator.New()
	//随后检查是否出现错误，如果有错误返回给客户端
	if data.ValidateMovie(v, movie); !v.Valid() {
		print(v.Errors)
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	//将数据存储到数据库中
	err = app.models.Movies.Insert(movie)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	fmt.Fprintf(w, "%+v", input)
}

func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil || id < 1 {
		app.notFoundResponse(w, r)
		return
	}
	movie, err := app.models.Movies.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	err = app.writeJSON(w, http.StatusOK, nil, envelope{"movie": movie})
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// 更新操作，先根据ID读取数据，没有返回，有的话再使用数据进行覆盖，最后返回给前端
func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
	//读取要更新的id
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	//根据ID查询
	movie, err := app.models.Movies.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	//解析json数据到对象中去
	var input struct {
		Title   *string       `json:"title"`
		Year    *int32        `json:"year"`
		Runtime *data.Runtime `json:"runtime"`
		Genres  []string      `json:"genres"`
	}
	//读取json数据
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	//将json数据更新到movie中
	if input.Title != nil {
		movie.Title = *input.Title
	}
	if input.Year != nil {
		movie.Year = *input.Year
	}
	if input.Runtime != nil {
		movie.Runtime = *input.Runtime
	}
	if input.Genres != nil {
		movie.Genres = input.Genres
	}
	//创建一个校验器校验数据
	v := validator.New()
	if data.ValidateMovie(v, movie); !v.Valid() {
		print(v.Errors)
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	//更新数据
	err = app.models.Movies.Update(movie)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	movie.Version = movie.Version + 1
	//将更新后的数据返回给前端
	err = app.writeJSON(w, http.StatusOK, nil, envelope{"movie": movie})
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	idParam, err := app.readIDParam(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	//根据ID取出数据
	movie, err := app.models.Movies.Get(idParam)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	//取出之后进行删除操作
	err = app.models.Movies.Delete(idParam)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	app.writeJSON(w, http.StatusOK, nil, envelope{"已删除数据": movie})
}

func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title  string
		Genres []string
		data.Filters
	}
	//初始化一个校验器
	v := validator.New()
	//调用Query获取url.Value map 包含了查询字符串数据
	values := r.URL.Query()
	input.Title = app.readString(values, "title", "")
	input.Genres = app.readCSV(values, "genres", []string{})

	input.Filters.Page = app.readInt(values, "page", 1, v)
	input.Filters.PageSize = app.readInt(values, "page_size", 20, v)

	input.Filters.Sort = app.readString(values, "sort", "id")
	input.Filters.SortSafeList = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	movies, metadata, err := app.models.Movies.GetAll(input.Title, input.Genres, input.Filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	err = app.writeJSON(w, http.StatusOK, nil, envelope{"metadata": metadata, "movies": movies})
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}
