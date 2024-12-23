package data

import (
	"Greenlight/internal/validator"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Movie struct {
	//如果想要使用omitempty而不更改键名，可以留空。例如这样`json:",omitempty"`逗号不可省略
	//omitempty指令在JSON输出中隐藏一个字段，当且仅当结构字段值为空时
	/*
		您也可以通过将struct字段设置为未导出来防止它出现在JSON输出中。但是使用json:“-”结构标记通常是一个更好的选择：
			这是一个明确的指示，防止未来有人没有意识到这个字段是不可导出的引发严重的后果
			如果你试图在未导出的字段上使用struct标记，旧版本的go vet会引发错误，但现在在go 1.16中已经修复了这个问题
	*/
	ID        int       `json:"id"`                // Unique integer ID for the movie
	CreatedAt time.Time `json:"-"`                 // Timestamp for when the movie is added to our database
	Title     string    `json:"title"`             // Movie title
	Year      int32     `json:"year,omitempty"`    // Movie release year
	Runtime   Runtime   `json:"runtime,omitempty"` // Movie runtime (in minutes)  运行时间，播放时长？
	Genres    []string  `json:"genres,omitempty"`  // Slice of genres for the movie (romance, comedy, etc.)
	Version   int32     `json:"version"`           // The version number starts at 1 and will be incremented each
	// time the movie information is updated
}

func ValidateMovie(v *validator.Validator, movie *Movie) {
	v.Check(movie.Title != "", "title", "must be provided")
	v.Check(len(movie.Title) <= 500, "title", "must not be more than 500 bytes long")
	v.Check(movie.Year != 0, "year", "must be provided")
	v.Check(movie.Year >= 1888, "year", "must be greater than 1888")
	v.Check(movie.Year <= int32(time.Now().Year()), "year", "must not be in the future")
	v.Check(movie.Runtime != 0, "runtime", "must be provided")
	v.Check(movie.Runtime > 0, "runtime", "must be a positive integer")
	v.Check(movie.Genres != nil, "genres", "must be provided")
	v.Check(len(movie.Genres) >= 1, "genres", "must contain at least 1 genre")
	v.Check(len(movie.Genres) <= 5, "genres", "must not contain more than 5 genres")
	v.Check(validator.Unique(movie.Genres), "genres", "must not contain duplicate values")
}

// MovieModelInterface 任何实现了以下四种方法的结构体都可以视为MovieModelInterface类型
type MovieModelInterface interface {
	Insert(movie *Movie) error
	Get(id int) (*Movie, error)
	Update(movie *Movie) error
	Delete(id int) error
	GetAll(title string, genres []string, f Filters) ([]*Movie, Metadata, error)
}

// MovieModel 用来封装所有数据库读写的代码
type MovieModel struct {
	DB *sql.DB
}

func (m *MovieModel) Insert(movie *Movie) error {
	//将字符串切片转换为一个字符串
	join := strings.Join(movie.Genres, ",")
	query := "insert into movies (title,year,runtime,genres,version,create_at)values (?,?,?,?,default,UTC_TIMESTAMP())"
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()
	result, err := m.DB.ExecContext(ctx, query, movie.Title, movie.Year, movie.Runtime, join)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	fmt.Printf("已成功更改数据%d", id)
	return nil
}

func (m *MovieModel) Get(id int) (*Movie, error) {
	query := "select * from movies where id = ?"
	//创建上下文
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	//上下文相关的资源执行完Get被释放，否则只有在三秒后或者父上下文才能被取消。
	defer cancelFunc()
	//在WithTimeOut创建上下文的那一刻与QueryRowContext之间的执行的代码也会记入超时
	//另一方面，超时会在查询执行之前发生，当sql连接池中的连接全部被占用时，查询会被排放在队列中等待，此时没有进行查询已经发生超时
	//上下文都有一个done channel，超时时done关闭，sql查询时，db会开启一个进程监听done。
	//此时数据库返回的大致错误为：
	row := m.DB.QueryRowContext(ctx, query, id)

	movie := &Movie{}
	var genres string
	//必须要按照数据库顺序写！！！
	err := row.Scan(&movie.ID, &movie.CreatedAt, &movie.Title, &movie.Year, &movie.Runtime, &genres, &movie.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	split := strings.Split(genres, ",")
	movie.Genres = split
	return movie, nil
}

func (m *MovieModel) Update(movie *Movie) error {
	//将切片转换为字符串存储
	join := strings.Join(movie.Genres, ",")
	query := "update movies set title=?, year=?, runtime=?, genres=?,version=version+1 where id=? and version = ?"
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()
	result, err := m.DB.ExecContext(ctx, query, movie.Title, movie.Year, movie.Runtime, join, movie.ID, movie.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrEditConflict
	}
	fmt.Printf("%d 条数据被更新\n", affected)
	return nil
}

// Delete 根据ID直接删除
func (m *MovieModel) Delete(id int) error {
	query := "delete from movies where id = ?"
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()
	result, err := m.DB.ExecContext(ctx, query, id)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrRecordNotFound
		default:
			return err
		}
	}
	affected, err := result.RowsAffected()
	fmt.Printf("%d 条数据被删除", affected)
	return nil
}

func (m *MovieModel) GetAll(title string, genres []string, f Filters) ([]*Movie, Metadata, error) {
	//当搜索条件为空，这个搜索条件被跳过
	//mysql没办法实现类似contain这样的函数，只能在外部进行操作。
	//查询语句无法将表的字段名用占位符的方式传递进去，因此需要使用Go语言将字符串拼接
	query := fmt.Sprintf("select SQL_CALC_FOUND_ROWS * from movies where(title like concat('%%',?,'%%') or ? = '') AND (FIND_IN_SET(?, genres)>0 or ? = '') order by %s %s, id ASC limit %d offset %d", f.sortColumn(), f.sortDirection(), f.limit(), f.offset())

	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFunc()
	join := strings.Join(genres, ",")
	stmt, err := m.DB.PrepareContext(ctx, query)
	if err != nil {
		return nil, Metadata{}, err
	}
	rows, err := stmt.Query(title, title, join, join)
	if err != nil {
		return nil, Metadata{}, err
	}
	var movies []*Movie
	for rows.Next() {
		var movie Movie
		var genres string
		rows.Scan(
			&movie.ID,
			&movie.CreatedAt,
			&movie.Title,
			&movie.Year,
			&movie.Runtime,
			&genres,
			&movie.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}
		//将genres转变为切片
		split := strings.Split(genres, ",")
		movie.Genres = split
		movies = append(movies, &movie)
	}
	//获取查询条目总数
	countQuery := "SELECT FOUND_ROWS()"
	var count int
	err = m.DB.QueryRow(countQuery).Scan(&count)
	if err != nil {
		return nil, Metadata{}, err
	}
	// rows.Err可以提取在循环中遇到的任何错误
	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}
	metadata := calculateMetadata(count, f.Page, f.PageSize)
	return movies, metadata, nil
}
