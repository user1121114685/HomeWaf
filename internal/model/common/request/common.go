package request

type PageInfo struct {
	PageIndex int    `json:"pageIndex" form:"pageIndex" binding:"omitempty,min=1,max=1000000"` //当前页面索引
	PageSize  int    `json:"pageSize" form:"pageSize" binding:"omitempty,min=1,max=1000"`      // 每页大小
	Keyword   string `json:"keyWord" form:"keyWord"`                                           //关键字
}
