package data

import (
	"Greenlight/internal/validator"
	"math"
	"strings"
)

type Filters struct {
	PageSize int
	Page     int
	Sort     string
	//装填支持的查询值类型
	SortSafeList []string
}

type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"` //omitempty标签使其在转换为json格式时忽略
	PageSize     int `json:"page_size,omitempty"`
	FirstPage    int `json:"first_page,omitempty"`
	LastPage     int `json:"last_page,omitempty"`
	TotalRecords int `json:"total_records,omitempty"`
}

func ValidateFilters(v *validator.Validator, f Filters) {
	v.Check(f.Page > 0, "page", "must be greater than zero")
	v.Check(f.Page <= 10_000_000, "page", "must be a maximum of 10 million")
	v.Check(f.PageSize > 0, "page_size", "must be greater than zero")
	v.Check(f.PageSize <= 100, "page_size", "must be a maximum of 100")

	v.Check(validator.In(f.Sort, f.SortSafeList...), "sort", "invalid sort value")
}
func (f Filters) sortColumn() string {
	for _, safeValue := range f.SortSafeList {
		if safeValue == f.Sort {
			return strings.TrimPrefix(f.Sort, "-")
		}
	}
	panic("unsafe sort parameter" + f.Sort)
}

// 返回排序模式
func (f Filters) sortDirection() string {
	if strings.HasPrefix(f.Sort, "-") {
		return "DESC"
	}
	return "ASC"
}
func (f Filters) limit() int {
	return f.PageSize
}
func (f Filters) offset() int {
	return (f.Page - 1) * f.PageSize
}

func calculateMetadata(totalRecord, page, pageSize int) Metadata {
	if totalRecord == 0 {
		return Metadata{}
	}
	return Metadata{
		CurrentPage:  page,
		PageSize:     pageSize,
		FirstPage:    1,
		LastPage:     int(math.Ceil(float64(totalRecord) / float64(pageSize))),
		TotalRecords: totalRecord,
	}
}
