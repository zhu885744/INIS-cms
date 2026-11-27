package controller

import (
	"inis/app/facade"
	"inis/app/model"
	"math"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

// 搜索参数边界
const (
	// searchKeywordMaxLen - 关键词最大长度（按 rune）
	// 更长的 LIKE 模式只会退化成全表扫描，对用户也没有实际意义
	searchKeywordMaxLen = 64
	// searchLimitDefault - 未传 limit 时的每页条数
	searchLimitDefault = 10
	// searchLimitMax - 单类结果每页上限（limit 传 10000 会把整表拉出来）
	searchLimitMax = 50
)

// Search - 搜索控制器
// @Summary 搜索管理API
// @Description 提供高效的全局搜索功能，支持多表搜索
// @Tags Search
type Search struct {
	// 继承
	base
}

// normalizeSearchKeyword - 规范化搜索关键词，并生成 LIKE 用的通配串
//
// 返回：
//   - keyword：清洗后的原始关键词（用于结果高亮与展示）
//   - term：转义过 LIKE 通配符、并加上首尾 % 的模式串（用于 SQL 参数）
//
// 必须转义的原因：用户输入 % 或 _ 会被 MySQL 当成通配符，
// 搜一个 "%" 就能匹配整张表（慢查询，还能借此探测未公开内容是否存在）。
func normalizeSearchKeyword(raw string) (keyword string, term string) {
	// 折叠首尾与内部连续空白：否则 " a  b " 会变成 %a  b%，实际内容里几乎命中不到
	keyword = strings.Join(strings.Fields(raw), " ")
	if keyword == "" {
		return "", ""
	}

	if runes := []rune(keyword); len(runes) > searchKeywordMaxLen {
		keyword = string(runes[:searchKeywordMaxLen])
	}

	// MySQL 默认以 \ 作为 LIKE 转义符：先把 \ 自身转义，再转义 % 与 _
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(keyword)

	return keyword, "%" + escaped + "%"
}

// clampSearchPage - 夹紧分页参数
//
// page 传负数会让 offset 变成负数；limit 传极大值等于把整表拉进内存。
func clampSearchPage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = searchLimitDefault
	}
	if limit > searchLimitMax {
		limit = searchLimitMax
	}
	return page, limit
}

// parseSearchArgs - 统一解析搜索入口的关键词与分页参数
//
// ok=false 表示已经写过 400 响应，调用方直接 return 即可。
func (this *Search) parseSearchArgs(ctx *gin.Context) (keyword string, page, limit int, ok bool) {
	params := this.params(ctx, map[string]any{
		"keyword": "",
		"page":    1,
		"limit":   searchLimitDefault,
	})

	keyword, _ = normalizeSearchKeyword(cast.ToString(params["keyword"]))
	if keyword == "" {
		this.json(ctx, nil, facade.Lang(ctx, "搜索关键词不能为空！"), 400)
		return "", 0, 0, false
	}

	page, limit = clampSearchPage(cast.ToInt(params["page"]), cast.ToInt(params["limit"]))
	return keyword, page, limit, true
}

// searchTable - 通用「多列 LIKE + 分页 + 总数」查询
//
// term 必须是 normalizeSearchKeyword 产出的已转义模式串；
// extraWhere 是追加的可见性条件（如 `audit = ?`），其占位符参数按顺序放在 extraArgs。
// 字段名统一加反引号：pages 的 key 这类保留字不加会直接语法错误。
func searchTable[T any](
	db *gorm.DB,
	term string,
	fields []string,
	extraWhere string,
	extraArgs []any,
	order string,
	page, limit int,
) ([]T, int64) {

	var items []T

	conditions := make([]string, 0, len(fields))
	args := make([]any, 0, len(fields)+len(extraArgs))
	for _, field := range fields {
		conditions = append(conditions, "`"+field+"` LIKE ?")
		args = append(args, term)
	}

	where := "(" + strings.Join(conditions, " OR ") + ")"
	if extraWhere != "" {
		where += " AND " + extraWhere
	}
	args = append(args, extraArgs...)

	base := db.Model(new(T)).Where(where, args...)

	var count int64
	if err := base.Session(&gorm.Session{}).Count(&count).Error; err != nil || count == 0 {
		return items, 0
	}

	_ = base.Order(order).Limit(limit).Offset((page - 1) * limit).Find(&items)

	return items, count
}

// highlightKeyword 关键词高亮（大小写不敏感）
func (this *Search) highlightKeyword(text string, keyword string) string {
	if text == "" || keyword == "" {
		return text
	}
	// 转义正则特殊字符；(?i) 让英文关键词大小写都能命中
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(keyword))
	return re.ReplaceAllString(text, `<mark>$0</mark>`)
}

// 搜索结果「内容预览」用的清理规则（见 cleanSearchText）
var (
	searchHTMLTagRe      = regexp.MustCompile(`<[^>]*>`)
	searchMarkdownImgRe  = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	searchMarkdownLinkRe = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	searchMarkdownLineRe = regexp.MustCompile(`(?m)^\s{0,3}(#{1,6}|>|[-*+]|\d+\.)\s+`)
	// 代码块围栏（含 ```go 这种语言标记）
	searchMarkdownFenceRe = regexp.MustCompile("(?m)^\\s*```.*$")
)

// cleanSearchText - 把正文整理成一段可读纯文本（用于搜索结果的摘要 / 内容预览）
//
// 只做轻量清理：去 HTML 标签、图片与链接语法、代码块围栏、行首 Markdown 标记，最后折叠空白。
// 目的是让「搜索结果里那一段话」看起来像正常文本，而不是一串标记。
func cleanSearchText(text string) string {
	if text == "" {
		return ""
	}

	text = searchHTMLTagRe.ReplaceAllString(text, " ")
	text = searchMarkdownImgRe.ReplaceAllString(text, " ")
	text = searchMarkdownLinkRe.ReplaceAllString(text, "$1")
	text = searchMarkdownFenceRe.ReplaceAllString(text, " ")
	text = searchMarkdownLineRe.ReplaceAllString(text, "")

	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}

// searchSnippetWindow - 从原文里截取「关键词附近」的一段窗口
//
// 只是为了让后续的纯文本化（5 次全文正则）与高亮工作在有限的文本上：
// 文章正文动辄几万字，列表 10 条就是 50 次全量正则替换，没有必要。
// size 为关键词前后各保留的字符数（按 rune，不会切坏中文）。
func searchSnippetWindow(content, keyword string, size int) string {
	if content == "" || size <= 0 {
		return content
	}

	runes := []rune(content)
	if len(runes) <= size*2 {
		return content
	}

	index := -1
	if keyword != "" {
		if byteIndex := strings.Index(strings.ToLower(content), strings.ToLower(keyword)); byteIndex >= 0 {
			index = len([]rune(content[:byteIndex]))
		}
	}
	if index < 0 {
		// 正文里没出现关键词（命中标题 / 标签）：取开头一段
		return string(runes[:size*2])
	}

	start := index - size
	if start < 0 {
		start = 0
	}
	end := index + size
	if end > len(runes) {
		end = len(runes)
	}

	return string(runes[start:end])
}

// snippetKeyword - 从正文里截取「包含关键词」的一段作为内容预览
//
// 返回的片段已做纯文本化处理，关键词用 <mark> 包裹（前端直接 v-html 展示即可）：
//   - 以关键词为锚点，前后各保留 before / after 个字符（按 rune 切，不会截断中文）；
//   - 截断处补省略号；
//   - 正文里找不到关键词（例如命中标题 / 标签）时，从开头截取。
func (this *Search) snippetKeyword(content string, keyword string, before, after int) string {
	// 先按关键词截窗口，避免对整篇正文做全量清理
	text := cleanSearchText(searchSnippetWindow(content, keyword, 3000))
	if text == "" {
		return ""
	}
	if before <= 0 {
		before = 40
	}
	if after <= 0 {
		after = 100
	}

	runes := []rune(text)
	keyRunes := []rune(keyword)

	// 用字节索引定位，再换算成 rune 索引，避免把多字节字符切坏
	index := -1
	if keyword != "" {
		if byteIndex := strings.Index(strings.ToLower(text), strings.ToLower(keyword)); byteIndex >= 0 {
			index = len([]rune(text[:byteIndex]))
		}
	}
	if index < 0 {
		index = 0
	}

	start := index - before
	if start < 0 {
		start = 0
	}
	end := index + len(keyRunes) + after
	if end > len(runes) {
		end = len(runes)
	}

	snippet := this.highlightKeyword(string(runes[start:end]), keyword)
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(runes) {
		snippet += "..."
	}

	return snippet
}

// highlightResult 高亮搜索结果中的关键词
func (this *Search) highlightResult(data []map[string]any, keyword string) []map[string]any {
	if keyword == "" {
		return data
	}

	// 需要高亮的字段
	highlightFields := []string{"title", "content", "abstract", "description", "name", "nickname"}

	for _, item := range data {
		for _, field := range highlightFields {
			if val, ok := item[field].(string); ok {
				item[field] = this.highlightKeyword(val, keyword)
			}
		}
	}
	return data
}

// searchFieldsWhitelist - 各搜索类型允许被检索的字段白名单
//
// fields 参数来自客户端（如 ?fields=nickname,email），历史上被直接拼进 SQL 的列名位置，
// 既可以用于布尔盲注（如 fields=id），也能探测未授权列（如 fields=password），
// 因此这里只放行白名单内的列，其余一律忽略并回退默认字段。
var searchFieldsWhitelist = map[string][]string{
	"article": {"title", "content", "abstract", "tags"},
	"pages":   {"title", "content", "key"},
	"tags":    {"name", "description"},
	"users":   {"nickname", "email", "description", "title"},
	"links":   {"nickname", "description", "url"},
	"moments": {"content", "location"},
}

// pickSearchFields - 从客户端入参中过滤出白名单内的检索字段；结果为空时回退默认字段
func (this *Search) pickSearchFields(kind string, fields []string, def []string) []string {
	if len(fields) == 0 || fields[0] == "" {
		return def
	}

	allow := searchFieldsWhitelist[kind]
	if len(allow) == 0 {
		return def
	}

	var picked []string
	for _, item := range strings.Split(fields[0], ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		for _, ok := range allow {
			if item == ok {
				picked = append(picked, item)
				break
			}
		}
	}

	if len(picked) == 0 {
		return def
	}

	return picked
}

// processSearchResult 处理搜索结果
func (this *Search) processSearchResult(items any, count int64, limit int, searchType string, keyword ...string) map[string]interface{} {
	var data []map[string]any

	// 内容预览要从正文里截，这里先把关键词取出来
	word := ""
	if len(keyword) > 0 {
		word = keyword[0]
	}

	switch v := items.(type) {
	case []model.Article:
		for _, article := range v {
			// snippet 为「内容预览」：摘要为空时前端会退化成展示这段正文片段。
			// 可见范围：非公开内容不输出正文片段，避免从搜索结果绕过登录 / 积分限制
			snippet := ""
			if model.ScopeNormalize(article.Scope) == model.ScopePublic {
				snippet = this.snippetKeyword(article.Content, word, 40, 100)
			}
			data = append(data, map[string]any{
				"id":             article.Id,
				"title":          article.Title,
				"covers":         article.Covers,
				"abstract":       article.Abstract,
				"snippet":        snippet,
				"create_time":    article.CreateTime,
				"tags":           article.Tags,
				"views":          article.Views,
				"audit":          article.Audit,
				"scope":          model.ScopeNormalize(article.Scope),
				"scope_integral": article.ScopeIntegral,
			})
		}
	case []model.Pages:
		for _, page := range v {
			snippet := ""
			if model.ScopeNormalize(page.Scope) == model.ScopePublic {
				snippet = this.snippetKeyword(page.Content, word, 40, 100)
			}
			data = append(data, map[string]any{
				"id":             page.Id,
				"key":            page.Key,
				"title":          page.Title,
				"snippet":        snippet,
				"create_time":    page.CreateTime,
				"views":          page.Views,
				"audit":          page.Audit,
				"scope":          model.ScopeNormalize(page.Scope),
				"scope_integral": page.ScopeIntegral,
			})
		}
	case []model.Tags:
		for _, tag := range v {
			data = append(data, map[string]any{
				"id":          tag.Id,
				"name":        tag.Name,
				"avatar":      tag.Avatar,
				"description": tag.Description,
			})
		}
	case []model.Users:
		for _, user := range v {
			data = append(data, map[string]any{
				"id":          user.Id,
				"nickname":    user.Nickname,
				"avatar":      user.Avatar,
				"description": user.Description,
				"title":       user.Title,
				"email":       model.HideContact(user.Email),
			})
		}
	case []model.Links:
		for _, link := range v {
			data = append(data, map[string]any{
				"id":          link.Id,
				"nickname":    link.Nickname,
				"avatar":      link.Avatar,
				"description": link.Description,
				"url":         link.Url,
				"audit":       link.Audit,
			})
		}
	case []model.Moments:
		for _, moment := range v {
			// 可见范围：非公开动态不输出正文 / 图片
			content, images := moment.Content, moment.Images
			if model.ScopeNormalize(moment.Scope) != model.ScopePublic {
				content, images = "", ""
			}
			data = append(data, map[string]any{
				"id":             moment.Id,
				"content":        content,
				"images":         images,
				"location":       moment.Location,
				"create_time":    moment.CreateTime,
				"audit":          moment.Audit,
				"status":         moment.Status,
				"scope":          model.ScopeNormalize(moment.Scope),
				"scope_integral": moment.ScopeIntegral,
			})
		}
	}

	// 高亮关键词
	if len(keyword) > 0 && keyword[0] != "" {
		data = this.highlightResult(data, keyword[0])
	}

	// 无结果时也要返回空数组而不是 null：前端直接 .length / v-for 会因 null 报错
	if data == nil {
		data = []map[string]any{}
	}

	if limit < 1 {
		limit = searchLimitDefault
	}

	return map[string]interface{}{
		"data":  data,
		"count": count,
		"page":  math.Ceil(float64(count) / float64(limit)),
		"type":  searchType,
	}
}

// IGET - GET请求本体
func (this *Search) IGET(ctx *gin.Context) {
	// 转小写
	method := strings.ToLower(ctx.Param("method"))

	allow := map[string]any{
		"article": this.article,
		"pages":   this.pages,
		"tags":    this.tags,
		"users":   this.users,
		"links":   this.links,
		"moments": this.moments,
		"all":     this.all,
	}
	err := this.call(allow, method, ctx)

	if err != nil {
		this.json(ctx, nil, facade.Lang(ctx, "方法调用错误：%v", err.Error()), 405)
		return
	}
}

// IPOST - POST请求本体
func (this *Search) IPOST(ctx *gin.Context) {
	this.json(ctx, nil, facade.Lang(ctx, "搜索控制器不支持POST请求"), 405)
}

// IPUT - PUT请求本体
func (this *Search) IPUT(ctx *gin.Context) {
	this.json(ctx, nil, facade.Lang(ctx, "搜索控制器不支持PUT请求"), 405)
}

// IDEL - DELETE请求本体
func (this *Search) IDEL(ctx *gin.Context) {
	this.json(ctx, nil, facade.Lang(ctx, "搜索控制器不支持DELETE请求"), 405)
}

// INDEX - 搜索首页
func (this *Search) INDEX(ctx *gin.Context) {
	this.json(ctx, map[string]interface{}{
		"message": "搜索控制器首页",
		"version": "1.0.0",
		"endpoints": map[string]string{
			"global":  "/api/search/all?keyword=关键词",
			"article": "/api/search/article?keyword=关键词",
			"pages":   "/api/search/pages?keyword=关键词",
			"tags":    "/api/search/tags?keyword=关键词",
			"users":   "/api/search/users?keyword=关键词",
			"links":   "/api/search/links?keyword=关键词",
			"moments": "/api/search/moments?keyword=关键词",
		},
	}, facade.Lang(ctx, "搜索控制器首页"), 200)
}

// article - 文章搜索
func (this *Search) article(ctx *gin.Context) {
	keyword, page, limit, ok := this.parseSearchArgs(ctx)
	if !ok {
		return
	}

	result := this.searchArticle(keyword, page, limit, cast.ToString(ctx.Query("fields")))
	result["keyword"] = keyword

	this.json(ctx, result, facade.Lang(ctx, "搜索成功！"), 200)
}

// pages - 页面搜索
func (this *Search) pages(ctx *gin.Context) {
	keyword, page, limit, ok := this.parseSearchArgs(ctx)
	if !ok {
		return
	}

	result := this.searchPages(keyword, page, limit, cast.ToString(ctx.Query("fields")))
	result["keyword"] = keyword

	this.json(ctx, result, facade.Lang(ctx, "搜索成功！"), 200)
}

// tags - 标签搜索
func (this *Search) tags(ctx *gin.Context) {
	keyword, page, limit, ok := this.parseSearchArgs(ctx)
	if !ok {
		return
	}

	result := this.searchTags(keyword, page, limit, cast.ToString(ctx.Query("fields")))
	result["keyword"] = keyword

	this.json(ctx, result, facade.Lang(ctx, "搜索成功！"), 200)
}

// users - 用户搜索
func (this *Search) users(ctx *gin.Context) {
	keyword, page, limit, ok := this.parseSearchArgs(ctx)
	if !ok {
		return
	}

	result := this.searchUsers(keyword, page, limit, cast.ToString(ctx.Query("fields")))
	result["keyword"] = keyword

	this.json(ctx, result, facade.Lang(ctx, "搜索成功！"), 200)
}

// links - 友链搜索
func (this *Search) links(ctx *gin.Context) {
	keyword, page, limit, ok := this.parseSearchArgs(ctx)
	if !ok {
		return
	}

	result := this.searchLinks(keyword, page, limit, cast.ToString(ctx.Query("fields")))
	result["keyword"] = keyword

	this.json(ctx, result, facade.Lang(ctx, "搜索成功！"), 200)
}

// moments - 动态搜索
func (this *Search) moments(ctx *gin.Context) {
	keyword, page, limit, ok := this.parseSearchArgs(ctx)
	if !ok {
		return
	}

	result := this.searchMoments(keyword, page, limit, cast.ToString(ctx.Query("fields")))
	result["keyword"] = keyword

	this.json(ctx, result, facade.Lang(ctx, "搜索成功！"), 200)
}

// all - 全局搜索
func (this *Search) all(ctx *gin.Context) {
	keyword, page, limit, ok := this.parseSearchArgs(ctx)
	if !ok {
		return
	}

	result := this.searchAll(keyword, page, limit)
	result["keyword"] = keyword

	this.json(ctx, result, facade.Lang(ctx, "搜索成功！"), 200)
}

// searchArticle - 搜索文章
//
// 只搜「已发布 + 已通过审核」：草稿（status=0）不该出现在前台搜索结果里。
func (this *Search) searchArticle(keyword string, page, limit int, fields ...string) map[string]interface{} {
	word, term := normalizeSearchKeyword(keyword)
	searchFields := this.pickSearchFields("article", fields, []string{"title", "content", "abstract", "tags"})

	items, count := searchTable[model.Article](
		facade.DB.Drive(), term, searchFields,
		"audit = ? AND status = ?", []any{1, 1},
		"create_time desc", page, limit,
	)

	return this.processSearchResult(items, count, limit, "article", word)
}

// searchPages - 搜索独立页面（独立页面没有草稿概念，只看审核状态）
func (this *Search) searchPages(keyword string, page, limit int, fields ...string) map[string]interface{} {
	word, term := normalizeSearchKeyword(keyword)
	searchFields := this.pickSearchFields("pages", fields, []string{"title", "content", "key"})

	items, count := searchTable[model.Pages](
		facade.DB.Drive(), term, searchFields,
		"audit = ?", []any{1},
		"create_time desc", page, limit,
	)

	return this.processSearchResult(items, count, limit, "pages", word)
}

// searchTags - 搜索标签
func (this *Search) searchTags(keyword string, page, limit int, fields ...string) map[string]interface{} {
	word, term := normalizeSearchKeyword(keyword)
	searchFields := this.pickSearchFields("tags", fields, []string{"name", "description"})

	items, count := searchTable[model.Tags](
		facade.DB.Drive(), term, searchFields,
		"", nil,
		"create_time desc", page, limit,
	)

	return this.processSearchResult(items, count, limit, "tags", word)
}

// searchUsers - 搜索用户
//
// status = 0 只搜正常账号：被封禁 / 冻结的用户不该在公开搜索里出现。
// email 命中结果在输出侧仍做脱敏（processSearchResult）。
func (this *Search) searchUsers(keyword string, page, limit int, fields ...string) map[string]interface{} {
	word, term := normalizeSearchKeyword(keyword)
	searchFields := this.pickSearchFields("users", fields, []string{"nickname", "email", "description", "title"})

	items, count := searchTable[model.Users](
		facade.DB.Drive(), term, searchFields,
		"status = ?", []any{0},
		"create_time desc", page, limit,
	)

	return this.processSearchResult(items, count, limit, "users", word)
}

// searchLinks - 搜索友链
func (this *Search) searchLinks(keyword string, page, limit int, fields ...string) map[string]interface{} {
	word, term := normalizeSearchKeyword(keyword)
	searchFields := this.pickSearchFields("links", fields, []string{"nickname", "description", "url"})

	items, count := searchTable[model.Links](
		facade.DB.Drive(), term, searchFields,
		"audit = ?", []any{1},
		"create_time desc", page, limit,
	)

	return this.processSearchResult(items, count, limit, "links", word)
}

// searchMoments - 搜索动态
//
// audit = 1 且 status = 1：草稿（status=0）和待审核内容都不该出现在公开搜索结果里。
func (this *Search) searchMoments(keyword string, page, limit int, fields ...string) map[string]interface{} {
	word, term := normalizeSearchKeyword(keyword)
	searchFields := this.pickSearchFields("moments", fields, []string{"content", "location"})

	items, count := searchTable[model.Moments](
		facade.DB.Drive(), term, searchFields,
		"audit = ? AND status = ?", []any{1, 1},
		"create_time desc", page, limit,
	)

	return this.processSearchResult(items, count, limit, "moments", word)
}

// searchAll - 全局搜索
func (this *Search) searchAll(keyword string, page, limit int) map[string]interface{} {
	perTypeLimit := limit / 6
	if perTypeLimit < 1 {
		perTypeLimit = 1
	}

	articleResult := this.searchArticle(keyword, page, perTypeLimit)
	pagesResult := this.searchPages(keyword, page, perTypeLimit)
	tagsResult := this.searchTags(keyword, page, perTypeLimit)
	usersResult := this.searchUsers(keyword, page, perTypeLimit)
	linksResult := this.searchLinks(keyword, page, perTypeLimit)
	momentsResult := this.searchMoments(keyword, page, perTypeLimit)

	return map[string]interface{}{
		"article": articleResult,
		"pages":   pagesResult,
		"tags":    tagsResult,
		"users":   usersResult,
		"links":   linksResult,
		"moments": momentsResult,
		"total":   cast.ToInt64(articleResult["count"]) + cast.ToInt64(pagesResult["count"]) + cast.ToInt64(tagsResult["count"]) + cast.ToInt64(usersResult["count"]) + cast.ToInt64(linksResult["count"]) + cast.ToInt64(momentsResult["count"]),
		"type":    "all",
	}
}
