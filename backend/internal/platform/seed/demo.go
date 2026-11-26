package seed

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/yanji-community/backend/internal/shared/markdown"
)

type demoUserSpec struct {
	Username    string
	Email       string
	DisplayName string
	Bio         string
	Website     string
	Location    string
	DaysAgo     int
	StreakDays  int
	Badges      []string
}

type demoTagSpec struct {
	Slug        string
	Name        string
	Description string
}

type demoCommentSpec struct {
	Key            string
	Author         string
	ParentKey      string
	QuoteKey       string
	Depth          int
	Body           string
	HoursAfterPost int
	Accepted       bool
	LikedBy        []string
}

type demoPollSpec struct {
	MultipleChoice bool
	MaxChoices     int
	AllowRevote    bool
	EndsInDays     int
	Options        []string
	Votes          map[string][]int // username -> option indexes
}

type demoPostSpec struct {
	Key          string
	Author       string
	CategorySlug string
	PostType     string
	Status       string
	Title        string
	Body         string
	Tags         []string
	IsPinned     bool
	IsFeatured   bool
	HoursAgo     int
	Views        int64
	LikedBy      []string
	BookmarkedBy []string
	Poll         *demoPollSpec
	Comments     []demoCommentSpec
}

type demoActivitySpec struct {
	Title        string
	Description  string
	Location     string
	StartsInDays int
	DurationHrs  int
	Capacity     int
	RewardPoints int
	Creator      string
	Participants []string
}

var demoUsers = []demoUserSpec{
	{
		Username:    "linmo",
		Email:       "linmo@demo.yanji.local",
		DisplayName: "林默",
		Bio:         "后端与数据库工程师，关注 PostgreSQL 查询调优、Go 服务可观测性与稳健架构。",
		Website:     "https://github.com/linmo-dev",
		Location:    "上海",
		DaysAgo:     45,
		StreakDays:  7,
		Badges:      []string{"first_post", "first_answer", "streak_7", "contributor"},
	},
	{
		Username:    "chenxi",
		Email:       "chenxi@demo.yanji.local",
		DisplayName: "陈曦",
		Bio:         "前端与交互开发者，偏好克制清晰的界面设计，日常写 TypeScript 与 Next.js。",
		Website:     "https://chenxi.design",
		Location:    "杭州",
		DaysAgo:     38,
		StreakDays:  4,
		Badges:      []string{"first_post", "contributor"},
	},
	{
		Username:    "zhouyuan",
		Email:       "zhouyuan@demo.yanji.local",
		DisplayName: "周远",
		Bio:         "开源工具爱好者，业余记录手冲咖啡参数、城市慢跑路线和读书笔记。",
		Website:     "",
		Location:    "成都",
		DaysAgo:     30,
		StreakDays:  3,
		Badges:      []string{"first_post"},
	},
	{
		Username:    "suxin",
		Email:       "suxin@demo.yanji.local",
		DisplayName: "苏昕",
		Bio:         "关注产品体验、效率工作流和团队协作方式，喜欢把复杂问题拆小。",
		Website:     "",
		Location:    "北京",
		DaysAgo:     26,
		StreakDays:  2,
		Badges:      []string{"first_post"},
	},
	{
		Username:    "tangwei",
		Email:       "tangwei@demo.yanji.local",
		DisplayName: "唐伟",
		Bio:         "运维与容器化实践者，专治 Docker Compose 网络、权限与证书疑难杂症。",
		Website:     "",
		Location:    "深圳",
		DaysAgo:     21,
		StreakDays:  2,
		Badges:      []string{"first_post"},
	},
}

var demoTags = []demoTagSpec{
	{Slug: "golang", Name: "Go 语言", Description: "Go 后端开发、并发模型与工程实践"},
	{Slug: "postgresql", Name: "PostgreSQL", Description: "关系型数据库设计、索引与查询调优"},
	{Slug: "nextjs", Name: "Next.js", Description: "React 全栈渲染、路由与前端性能"},
	{Slug: "docker", Name: "Docker", Description: "容器构建、Compose 编排与线上运维"},
	{Slug: "architecture", Name: "系统架构", Description: "模块化设计、缓存策略与可维护性"},
	{Slug: "productivity", Name: "效率工具", Description: "工作流自动化、笔记整理与开发工具"},
	{Slug: "coffee", Name: "咖啡日常", Description: "手冲器具、豆单风味与制作记录"},
	{Slug: "lifestyle", Name: "生活方式", Description: "运动健康、阅读习惯与日常记录"},
	{Slug: "feedback", Name: "社区建议", Description: "功能建议、使用体验反馈与社区规则讨论"},
}

var demoPosts = []demoPostSpec{
	{
		Key:          "guide",
		Author:       "admin",
		CategorySlug: "site",
		PostType:     "discussion",
		Status:       "published",
		Title:        "【社区指南】研记社区发帖规范、积分等级与功能导览",
		Tags:         []string{"feedback"},
		IsPinned:     true,
		IsFeatured:   true,
		HoursAgo:     144,
		Views:        486,
		LikedBy:      []string{"linmo", "chenxi", "zhouyuan", "suxin", "tangwei"},
		BookmarkedBy: []string{"linmo", "chenxi", "suxin"},
		Body: `欢迎来到研记社区。为了让大家在讨论技术与分享日常时都能保持清晰高效的阅读体验，整理了一份简要导览。

### 1. 版块划分

- **综合讨论**：适合工具推荐、工作方式、跨领域话题与投票征集
- **技术分享**：编程语言、数据库、前端工程、运维部署与排错实录
- **生活日常**：读书、运动、咖啡饮食、摄影等真实生活记录
- **站务公告**：版本更新说明、活动通知与规则反馈

### 2. 帖子类型

在发帖页顶部可以根据内容形式选择最合适的一类：

1. **讨论 / 长文章**：支持完整 Markdown 语法、表格与代码块高亮。
2. **问答**：发布后可在评论区将最有效的回复**采纳为最佳答案**，回答者会自动获得积分与勋章。
3. **投票**：支持单选或多选，并可设置截止时间与是否允许改票。

### 3. 积分与等级一览

| 行为 | 积分变化 | 备注 |
| --- | --- | --- |
| 每日签到 | +5 | 连续签到 7 天可解锁「七日之约」勋章 |
| 发布帖子 | +10 | 首次发帖自动解锁「开笔之喜」勋章 |
| 内容获赞 | +1 | 每篇帖子或评论首次被点赞时发放 |
| 回答被采纳 | +20 | 仅问答帖支持采纳顶层回复 |

> 如果在使用过程中发现界面细节、验证码、通知或权限问题，欢迎随时在「站务公告」或「社区建议」标签下留言。`,
		Comments: []demoCommentSpec{
			{
				Key:            "g1",
				Author:         "linmo",
				Depth:          0,
				HoursAfterPost: 2,
				Body:           "表格里的等级和积分规则很清楚，新注册的小伙伴记得先完成邮箱验证再来发帖，另外右侧栏的每日签到入口也很顺手。",
				LikedBy:        []string{"admin", "chenxi"},
			},
			{
				Key:            "g2",
				Author:         "chenxi",
				ParentKey:      "g1",
				Depth:          1,
				HoursAfterPost: 4,
				Body:           "同意，长代码块右上角还自带了一键复制按钮，写排错教程体验不错。",
				LikedBy:        []string{"linmo"},
			},
			{
				Key:            "g3",
				Author:         "zhouyuan",
				Depth:          0,
				HoursAfterPost: 9,
				Body:           "建议后面「生活日常」版块也可以考虑支持图片相册排版，日常发咖啡和跑步打卡会更直观。",
				LikedBy:        []string{"suxin"},
			},
			{
				Key:            "g4",
				Author:         "admin",
				ParentKey:      "g3",
				QuoteKey:       "g3",
				Depth:          1,
				HoursAfterPost: 11,
				Body:           "收到，已记入后续迭代清单，当前可以先通过正文插入上传图片使用。",
				LikedBy:        []string{"zhouyuan", "linmo"},
			},
		},
	},
	{
		Key:          "pg18",
		Author:       "linmo",
		CategorySlug: "tech",
		PostType:     "article",
		Status:       "published",
		Title:        "PostgreSQL 18 在社区场景下的索引与中文检索实战总结",
		Tags:         []string{"postgresql", "golang", "architecture"},
		IsPinned:     false,
		IsFeatured:   true,
		HoursAgo:     118,
		Views:        362,
		LikedBy:      []string{"admin", "chenxi", "tangwei", "suxin"},
		BookmarkedBy: []string{"chenxi", "tangwei"},
		Body: `最近在重构论坛的列表筛选与搜索接口时，顺手把 PostgreSQL 18 的几项常用优化落地到生产库里，这里记录一下踩坑和验证结果。

### 为什么 ` + "`simple`" + ` 分词对中英混排不够用

很多项目习惯直接对 ` + "`title || body`" + ` 建 ` + "`to_tsvector('simple', ...)`" + ` 的 GIN 索引。遇到英文单词、Go 包名（如 ` + "`pgxpool`" + `）时命中很准，但如果用户只搜中文短语（比如“连接池”或“验证码”），没有外部分词插件时 ` + "`simple`" + ` 会把整段连续汉字当成一个词元，导致漏检。

我们的做法是**双轨并行**：

` + "```sql" + `
-- 1. 英文关键词走 STORED tsvector + GIN 索引
CREATE INDEX idx_posts_tsv ON posts USING GIN (tsv)
  WHERE status = 'published' AND deleted_at IS NULL;

-- 2. 中文短语兜底走 pg_trgm 索引
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_posts_title_trgm ON posts USING GIN (title gin_trgm_ops);
CREATE INDEX idx_posts_body_trgm ON posts USING GIN (body_markdown gin_trgm_ops);
` + "```" + `

### 计数字段千万不要靠定时全表扫

像 ` + "`like_count`" + `、` + "`comment_count`" + `、` + "`categories.post_count`" + ` 这种列表高频展示字段，尽量在事务里结合唯一约束和原子 SQL 更新：

1. 点赞表以 ` + "`(user_id, post_id)`" + ` 作联合主键，配合 ` + "`ON CONFLICT DO NOTHING`" + `。
2. 只有当 ` + "`RowsAffected() > 0`" + ` 时才执行 ` + "`UPDATE posts SET like_count = like_count + 1`" + `。
3. 删除或状态切换（例如从 ` + "`published`" + ` 变为 ` + "`hidden`" + `）统一通过 ` + "`wasVisible / willVisible`" + ` 判定，避免重复加减导致计数漂移。

### 实测效果

在 5 万条模拟帖子样本下，热门信息流首屏耗时稳定在 ` + "`4ms`" + ` 以内，中文关键词 ` + "`ILIKE`" + ` 在 ` + "`gin_trgm_ops`" + ` 支撑下基本无需回表全扫。`,
		Comments: []demoCommentSpec{
			{
				Key:            "p1",
				Author:         "tangwei",
				Depth:          0,
				HoursAfterPost: 3,
				Body:           "补充一点，`pg_trgm` 对 3 个字符以上的中文片段走索引最稳；如果是单字或双字搜索，数据量不大时顺序扫描配合部分索引反而更快。",
				LikedBy:        []string{"linmo", "chenxi"},
			},
			{
				Key:            "p2",
				Author:         "linmo",
				ParentKey:      "p1",
				Depth:          1,
				HoursAfterPost: 5,
				Body:           "没错，所以我在服务端限制了搜索词太短时的兜底策略，并把软删除条件放进了部分索引里缩小索引体积。",
				LikedBy:        []string{"tangwei"},
			},
			{
				Key:            "p3",
				Author:         "chenxi",
				ParentKey:      "p2",
				QuoteKey:       "p2",
				Depth:          2,
				HoursAfterPost: 7,
				Body:           "前端搜索框里也做了防抖和快捷键 `/` 聚焦，联调起来整条链路很流畅。",
				LikedBy:        []string{"linmo"},
			},
		},
	},
	{
		Key:          "nextjs",
		Author:       "chenxi",
		CategorySlug: "tech",
		PostType:     "article",
		Status:       "published",
		Title:        "Next.js App Router 社区前端实践：布局克制感与状态三态处理",
		Tags:         []string{"nextjs", "architecture"},
		IsPinned:     false,
		IsFeatured:   true,
		HoursAgo:     92,
		Views:        274,
		LikedBy:      []string{"linmo", "suxin", "zhouyuan"},
		BookmarkedBy: []string{"linmo", "suxin"},
		Body: `做社区类产品最怕两件事：一是页面堆满花哨的渐变阴影，越看越疲劳；二是接口一报错或网络慢就只剩白屏。

这次改版前端时，主要坚持了下面几条原则：

### 统一 Design Tokens，收敛色彩噪音

全站所有边距、圆角、阴影、字体与主题色全部收敛到 CSS Variables（` + "`tokens.css`" + `），深浅色模式只改 token 不改组件逻辑。主按钮和链接使用克制的品牌蓝与中性灰阶，把视觉重心让渡给标题和正文内容。

### 数据请求收敛与完整三态

` + "```tsx" + `
if (feed.isLoading) return <ListSkeleton count={4} />;
if (feed.error) return <ErrorState onRetry={() => feed.refetch()} />;
if (items.length === 0) return <EmptyState title="还没有内容" />;
` + "```" + `

- **服务端状态统一交给 TanStack Query**：列表分页、详情缓存、点赞乐观/失效刷新都用同一套 queryKey 管理。
- **跨页轻量状态再进 Zustand**：比如当前登录会话摘要和编辑器未提交草稿。
- **表单提交防连点**：所有发帖、评论、登录注册按钮统一绑定 ` + "`pending`" + ` 状态，避免网络波动时重复发请求。`,
		Comments: []demoCommentSpec{
			{
				Key:            "n1",
				Author:         "suxin",
				Depth:          0,
				HoursAfterPost: 4,
				Body:           "克制的视觉层次真的很重要，信息密度高但留白舒服，看长文章不会晃眼。",
				LikedBy:        []string{"chenxi"},
			},
			{
				Key:            "n2",
				Author:         "zhouyuan",
				Depth:          0,
				HoursAfterPost: 10,
				Body:           "深色模式对比度调得也不错，晚上看不会刺眼。",
				LikedBy:        []string{"chenxi"},
			},
		},
	},
	{
		Key:          "docker-q",
		Author:       "tangwei",
		CategorySlug: "tech",
		PostType:     "question",
		Status:       "published",
		Title:        "Docker Compose 环境下 Go 服务连接 PostgreSQL 和健康检查怎么配更稳妥？",
		Tags:         []string{"docker", "golang", "postgresql"},
		IsPinned:     false,
		IsFeatured:   false,
		HoursAgo:     80,
		Views:        219,
		LikedBy:      []string{"linmo", "chenxi", "admin"},
		BookmarkedBy: []string{"admin", "chenxi"},
		Body: `单机用 Docker Compose 同时起 ` + "`postgres`" + `、` + "`redis`" + `、` + "`api`" + ` 和 ` + "`web`" + ` 时，发现一个常见问题：

虽然 Compose 里配了 ` + "`depends_on`" + `，但有时数据库容器刚起来、正在做初始恢复或执行迁移脚本，` + "`api`" + ` 容器就立刻连过去并因为 ` + "`connection refused`" + ` 直接退出重启了。

大家在生产环境里一般怎么处理这几件事？
1. Compose 健康检查间隔和重试次数怎么设比较合理？
2. ` + "`pgxpool`" + ` 的 ` + "`MaxConns`" + ` 和连接保活参数建议配多少？`,
		Comments: []demoCommentSpec{
			{
				Key:            "d1",
				Author:         "linmo",
				Depth:          0,
				HoursAfterPost: 1,
				Accepted:       true,
				Body: `建议从**编排启动顺序**和**连接池自愈**两层一起做：

1. **Compose 必须等 ` + "`service_healthy`" + `**：
` + "```yaml" + `
depends_on:
  postgres:
    condition: service_healthy
healthcheck:
  test: ["CMD-SHELL", "pg_isready -U $${POSTGRES_USER} -d $${POSTGRES_DB}"]
  interval: 5s
  timeout: 5s
  retries: 10
` + "```" + `
注意首次初始化卷时 ` + "`pg_isready`" + ` 可能会短暂返回一次 ready 再重启，因此在 Go 进程启动连库时也可以加个 3~5 次指数退避重试。

2. **` + "`pgxpool`" + ` 参数参考**（2C4G 单机）：
- ` + "`MaxConns`" + ` 设为 ` + "`15 ~ 25`" + ` 足够，避免和 PostgreSQL 默认 100 连接打满。
- ` + "`MaxConnIdleTime = 10 * time.Minute`" + `、` + "`HealthCheckPeriod = 30 * time.Second`" + `。

3. **Nginx 对 API / Web 的反代超时**记得把 ` + "`proxy_read_timeout`" + ` 调到 ` + "`60s`" + `，同时注意 SSE 通知流需要关闭 ` + "`proxy_buffering off`" + `。`,
				LikedBy: []string{"tangwei", "chenxi", "admin"},
			},
			{
				Key:            "d2",
				Author:         "tangwei",
				ParentKey:      "d1",
				Depth:          1,
				HoursAfterPost: 3,
				Body:           "非常实用，尤其是首次初始化卷后 `pg_isready` 短暂假阳性的坑我之前就踩过，加上启动重试后彻底稳了，已采纳为最佳回答。",
				LikedBy:        []string{"linmo"},
			},
			{
				Key:            "d3",
				Author:         "chenxi",
				Depth:          0,
				HoursAfterPost: 6,
				Body:           "SSE 关闭 `proxy_buffering` 那条太关键了，不然实时通知会在 Nginx 缓冲区里卡半天。",
				LikedBy:        []string{"tangwei", "linmo"},
			},
		},
	},
	{
		Key:          "poll-share",
		Author:       "suxin",
		CategorySlug: "general",
		PostType:     "poll",
		Status:       "published",
		Title:        "第一期社区线上分享会，大家最想听哪两个方向的内容？",
		Tags:         []string{"productivity", "feedback"},
		IsPinned:     false,
		IsFeatured:   false,
		HoursAgo:     68,
		Views:        195,
		LikedBy:      []string{"linmo", "chenxi", "zhouyuan", "tangwei"},
		BookmarkedBy: []string{"linmo"},
		Body: `准备在下周五晚上组织一场 60 分钟的轻量化线上交流分享，不念 PPT，主要做真实案例拆解和自由问答。

本次投票**最多可选 2 项**，也欢迎直接在评论区补充你想了解的具体问题，方便嘉宾提前准备实操演示。`,
		Poll: &demoPollSpec{
			MultipleChoice: true,
			MaxChoices:     2,
			AllowRevote:    true,
			EndsInDays:     14,
			Options: []string{
				"Go 并发模型与线上性能剖析（pprof 实战）",
				"PostgreSQL 慢查询排查与索引优化",
				"Next.js 前端交互细节与样式系统搭建",
				"个人知识管理、效率工具与自动化工作流",
			},
			Votes: map[string][]int{
				"linmo":    {0, 1},
				"chenxi":   {2, 0},
				"zhouyuan": {3, 1},
				"tangwei":  {0, 1},
				"suxin":    {2, 3},
			},
		},
		Comments: []demoCommentSpec{
			{
				Key:            "ps1",
				Author:         "linmo",
				Depth:          0,
				HoursAfterPost: 4,
				Body:           "投了 pprof 和慢查询排查，如果选第二个方向我可以带上几个真实脱敏执行计划来一起看。",
				LikedBy:        []string{"suxin", "tangwei"},
			},
			{
				Key:            "ps2",
				Author:         "suxin",
				ParentKey:      "ps1",
				Depth:          1,
				HoursAfterPost: 5,
				Body:           "太好了！大家有平时遇到的慢 SQL 也可以提前脱敏贴过来。",
			},
		},
	},
	{
		Key:          "nginx-sec",
		Author:       "tangwei",
		CategorySlug: "tech",
		PostType:     "discussion",
		Status:       "published",
		Title:        "单机 Docker + Nginx 部署全栈项目时的几点基础安全加固建议",
		Tags:         []string{"docker", "architecture"},
		IsPinned:     false,
		IsFeatured:   false,
		HoursAgo:     52,
		Views:        173,
		LikedBy:      []string{"admin", "linmo", "chenxi"},
		BookmarkedBy: []string{"linmo", "admin"},
		Body: `项目跑通上线只是第一步，单台云主机暴露在公网上很快就会迎来各种扫描器探测。分享几项成本最低但最管用的加固清单：

1. **数据库和 Redis 端口只监听 ` + "`127.0.0.1`" + `**：永远不要把 ` + "`5432`" + ` 或 ` + "`6379`" + ` 直接映射到 ` + "`0.0.0.0`" + `，只让同网段容器内网互访。
2. **认证入口加图形验证码与 Redis 限流**：针对登录、注册、忘记密码等接口做每分钟频控与一次性验证码，防撞库和恶意刷注册。
3. **Cookie 属性与跨域策略**：生产启用 HTTPS 后及时打开 ` + "`HttpOnly + SameSite + Secure`" + `，避免前端 JavaScript 读取敏感凭证。
4. **上传文件校验 Magic Bytes**：不要只相信扩展名和 ` + "`Content-Type`" + `，后端应读取文件头魔数并生成随机存储名。

大家还有哪些顺手的防护习惯，欢迎一起补充。`,
		Comments: []demoCommentSpec{
			{
				Key:            "ns1",
				Author:         "admin",
				Depth:          0,
				HoursAfterPost: 3,
				Body:           "很扎实的清单，我们目前注册、登录和忘记密码都已经接入一次性的图形验证码和频控限制，后续上域名 HTTPS 后会同步开启 Secure Cookie。",
				LikedBy:        []string{"tangwei", "linmo"},
			},
		},
	},
	{
		Key:          "coffee",
		Author:       "zhouyuan",
		CategorySlug: "life",
		PostType:     "article",
		Status:       "published",
		Title:        "在家做手冲咖啡半年：磨豆机、水温与记录表带来的改变",
		Tags:         []string{"coffee", "productivity", "lifestyle"},
		IsPinned:     false,
		IsFeatured:   true,
		HoursAgo:     40,
		Views:        248,
		LikedBy:      []string{"suxin", "chenxi", "linmo", "admin"},
		BookmarkedBy: []string{"suxin", "chenxi"},
		Body: `从半年前入手第一套 V60 滤杯开始，几乎每天早上工作前都会花 6 分钟冲一杯手冲。一开始常常今天偏酸、明天过苦，稳定下来其实只靠做了三件小事：

### 1. 先把注水节奏固定，再调研磨度

以前总爱边冲边换水流和绕圈方式，出问题根本不知道原因出在哪。后来干脆固定成简单的三段式：
- ` + "`0:00`" + `：30g 水闷蒸 35 秒
- ` + "`0:35`" + `：中心小水流均匀注到 140g
- ` + "`1:15`" + `：补水至 225g，2:05 左右滴滤完成

### 2. 记录豆单、水温与体感评分

| 产区/处理法 | 粉水比 | 水温 | 研磨刻度 | 风味体感 |
| --- | --- | --- | --- | --- |
| 埃塞耶加雪菲 水洗 | 1:15 | 92°C | 22 格 | 柑橘与白花香干净，尾韵带甜 |
| 哥伦比亚 厌氧日晒 | 1:15 | 89°C | 24 格 | 水温太高容易发涩，降到 89°C 刚好 |
| 肯尼亚 SL28 水洗 | 1:16 | 93°C | 21 格 | 莓果酸质饱满，适合放微凉再喝 |

### 3. 别迷信昂贵器具，先升级一把均匀的磨豆机

对比下来，滤杯和分享壶差异远不如磨豆机粒径分布来得明显。细粉少了一大半之后，后段杂味少很多。`,
		Comments: []demoCommentSpec{
			{
				Key:            "c1",
				Author:         "suxin",
				Depth:          0,
				HoursAfterPost: 5,
				Body:           "深烘和厌氧豆降到 88~89°C 真的立竿见影，以前 92°C 一路冲到底老觉得发苦。",
				LikedBy:        []string{"zhouyuan"},
			},
			{
				Key:            "c2",
				Author:         "chenxi",
				Depth:          0,
				HoursAfterPost: 12,
				Body:           "这个记录表格写得太清晰了，周末准备按照这个水温和粉水比试试耶加雪菲。",
				LikedBy:        []string{"zhouyuan", "suxin"},
			},
		},
	},
	{
		Key:          "tools",
		Author:       "suxin",
		CategorySlug: "general",
		PostType:     "discussion",
		Status:       "published",
		Title:        "聊聊大家日常写代码和整理笔记时，最顺手的效率工具组合",
		Tags:         []string{"productivity", "lifestyle"},
		IsPinned:     false,
		IsFeatured:   false,
		HoursAgo:     28,
		Views:        142,
		LikedBy:      []string{"chenxi", "zhouyuan", "linmo"},
		BookmarkedBy: []string{"zhouyuan"},
		Body: `用过一大圈笔记软件和效率工具之后，最近越来越倾向做减法：少装插件，少折腾外观，能快速检索和长期保存本地文本就行。

我现在的日常主力是：
- **长文与知识库**：纯 Markdown 目录 + Git 版本控制，换编辑器也不受制于格式。
- **临时灵感与待办**：系统自带备忘录配合快捷键呼出，不给零碎想法加分类负担。
- **终端工具**：` + "`ripgrep`" + ` + ` + "`fzf`" + ` + ` + "`tmux`" + `，几乎解决 80% 的文件查找与多任务切换。

大家目前保留下来用得最久的工具是什么？`,
		Comments: []demoCommentSpec{
			{
				Key:            "t1",
				Author:         "linmo",
				Depth:          0,
				HoursAfterPost: 4,
				Body:           "`ripgrep` 真是离开就难受的工具，大仓库秒搜。笔记我也是纯 Markdown 放本地目录，定期打包备份。",
				LikedBy:        []string{"suxin"},
			},
			{
				Key:            "t2",
				Author:         "zhouyuan",
				ParentKey:      "t1",
				Depth:          1,
				HoursAfterPost: 6,
				Body:           "+1，纯文本配合全文搜索比套十层标签好用太多，真正需要时搜一下就出来。",
			},
		},
	},
	{
		Key:          "running-q",
		Author:       "zhouyuan",
		CategorySlug: "general",
		PostType:     "question",
		Status:       "published",
		Title:        "最近夜跑 5 公里后膝盖外侧偶尔微酸，是跑量加太快还是跑鞋磨损了？",
		Tags:         []string{"lifestyle"},
		IsPinned:     false,
		IsFeatured:   false,
		HoursAgo:     16,
		Views:        98,
		LikedBy:      []string{"suxin", "tangwei"},
		BookmarkedBy: []string{},
		Body: `这三周把每次夜跑从 3 公里慢慢加到了 5 公里，配速大概 6 分半左右，前两周都很轻松，这几天跑完步走楼梯时感觉膝盖外侧有点酸胀。

平时跑前只简单压腿两分钟，鞋穿了快两年、大底外侧磨损有一点明显。想请教有长跑经验的同学，一般先排查哪几项比较好？`,
		Comments: []demoCommentSpec{
			{
				Key:            "r1",
				Author:         "tangwei",
				Depth:          0,
				HoursAfterPost: 3,
				Body:           "膝盖外侧酸胀很常见，先别硬撑着连续跑。可以留意两个点：1）步频提高到 175 以上减小落地冲击；2）跑前加上臀中肌激活和髋关节热身，跑鞋外底明显磨偏也建议尽快更换。",
				LikedBy:        []string{"zhouyuan", "suxin"},
			},
			{
				Key:            "r2",
				Author:         "zhouyuan",
				ParentKey:      "r1",
				Depth:          1,
				HoursAfterPost: 5,
				Body:           "感谢提醒！我平时步频确实只有 160 左右，跨步偏大，这周先减量休息两天再调整热身动作看看。",
			},
		},
	},
	{
		Key:          "poll-css",
		Author:       "chenxi",
		CategorySlug: "tech",
		PostType:     "poll",
		Status:       "published",
		Title:        "日常开发 Web 界面时，你现在最顺手的样式方案是哪一种？",
		Tags:         []string{"nextjs", "architecture"},
		IsPinned:     false,
		IsFeatured:   false,
		HoursAgo:     9,
		Views:        116,
		LikedBy:      []string{"linmo", "suxin"},
		BookmarkedBy: []string{},
		Body:         `每个项目对组件库、可定制性和构建体积的诉求不太一样，好奇大家目前实际项目里用得最多的主力样式方案。`,
		Poll: &demoPollSpec{
			MultipleChoice: false,
			MaxChoices:     1,
			AllowRevote:    true,
			EndsInDays:     7,
			Options: []string{
				"CSS Variables（Tokens）+ 轻量化基础样式",
				"Tailwind CSS 原子类",
				"CSS Modules / Vanilla Extract",
				"Ant Design / UI 组件库为主",
			},
			Votes: map[string][]int{
				"chenxi":   {0},
				"linmo":    {1},
				"suxin":    {0},
				"tangwei":  {3},
				"zhouyuan": {1},
			},
		},
		Comments: []demoCommentSpec{
			{
				Key:            "pc1",
				Author:         "chenxi",
				Depth:          0,
				HoursAfterPost: 2,
				Body:           "我们现在前台用 CSS Tokens + 语义类保证设计克制统一，管理后台用 Ant Design 提效，两边各取所长。",
				LikedBy:        []string{"linmo"},
			},
		},
	},
	{
		Key:          "locked-notice",
		Author:       "admin",
		CategorySlug: "site",
		PostType:     "discussion",
		Status:       "locked",
		Title:        "2026 年 11 月系统安全升级与注册登录验证码上线公告",
		Tags:         []string{"feedback"},
		IsPinned:     false,
		IsFeatured:   false,
		HoursAgo:     3,
		Views:        84,
		LikedBy:      []string{"linmo", "chenxi", "tangwei"},
		BookmarkedBy: []string{},
		Body: `为提升账号与服务安全性，本周已完成以下升级维护：

- 登录、注册与找回密码流程全量接入图形验证码（` + "`GET /api/v1/auth/captcha`" + `），一次性消费、区分容错并防暴力试探。
- 调整反向代理及接口安全策略，敏感操作均写入系统审计日志。
- 优化部分接口事务与类型扫描稳定性。

本公告仅作存档通知，已设为锁定状态；如对新功能有其他建议，请新开反馈帖说明。`,
		Comments: []demoCommentSpec{
			{
				Key:            "ln1",
				Author:         "linmo",
				Depth:          0,
				HoursAfterPost: 1,
				Body:           "验证码图片加载很快，字符清晰不费眼，点赞。",
				LikedBy:        []string{"admin"},
			},
		},
	},
}

var demoActivities = []demoActivitySpec{
	{
		Title:        "研记社区第一期线上沙龙：全栈工程实战与性能调优拆解",
		Description:  "本次线上沙龙将围绕 Go 模块化单体服务、PostgreSQL 18 索引优化与 Next.js 前端状态管理展开，采用 40 分钟实操分享 + 20 分钟自由问答形式。\n\n出席并完成签到后，系统将自动发放活动奖励积分。",
		Location:     "线上语音 / 视频会议（报名后站内通知发送链接）",
		StartsInDays: 5,
		DurationHrs:  2,
		Capacity:     50,
		RewardPoints: 30,
		Creator:      "admin",
		Participants: []string{"linmo", "chenxi", "suxin", "tangwei"},
	},
	{
		Title:        "周末城市慢跑与手冲咖啡线下交流局",
		Description:  "轻松的周末上午线下小聚：先沿河道慢跑 4~5 公里拉伸放松，随后在咖啡馆聊聊最近在读的书、效率工具与各自折腾的小项目。\n\n欢迎新手加入，重在交流不拼配速。",
		Location:     "成都 · 玉林路咖啡馆",
		StartsInDays: 9,
		DurationHrs:  3,
		Capacity:     20,
		RewardPoints: 20,
		Creator:      "zhouyuan",
		Participants: []string{"zhouyuan", "suxin", "chenxi"},
	},
	{
		Title:        "新版本体验建议与界面细节打磨征集周",
		Description:  "集中收集大家在首页浏览、编辑器发帖、私信与通知中心使用过程中的改进意见，优质建议采纳后将发放社区贡献积分。",
		Location:     "社区站务讨论区",
		StartsInDays: 2,
		DurationHrs:  12,
		Capacity:     0,
		RewardPoints: 15,
		Creator:      "admin",
		Participants: []string{"linmo", "chenxi"},
	},
}

// RunDemo 写入幂等演示数据（用户、标签、帖子、楼中楼、投票、活动、积分与勋章）。
func RunDemo(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	userIDs, adminID, err := ensureDemoUsersTx(ctx, tx)
	if err != nil {
		return fmt.Errorf("demo users: %w", err)
	}
	if err := ensureDemoTagsTx(ctx, tx); err != nil {
		return fmt.Errorf("demo tags: %w", err)
	}
	if err := ensureDemoFollowsTx(ctx, tx, userIDs, adminID); err != nil {
		return fmt.Errorf("demo follows: %w", err)
	}

	postIDs, newPosts, acceptedCommentIDs, err := ensureDemoPostsTx(ctx, tx, userIDs)
	if err != nil {
		return fmt.Errorf("demo posts: %w", err)
	}
	if err := ensureDemoActivitiesAndAnnouncementsTx(ctx, tx, userIDs, adminID); err != nil {
		return fmt.Errorf("demo activities: %w", err)
	}
	if err := ensureDemoPointsAndBadgesTx(ctx, tx, userIDs, adminID, postIDs, acceptedCommentIDs); err != nil {
		return fmt.Errorf("demo points: %w", err)
	}
	if err := recountDemoMetricsTx(ctx, tx, postIDs); err != nil {
		return fmt.Errorf("demo recount: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	logger.Info("demo seed ready", "users", len(demoUsers), "posts", len(postIDs), "new_posts", len(newPosts))
	return nil
}

func ensureDemoUsersTx(ctx context.Context, tx pgx.Tx) (map[string]int64, int64, error) {
	userIDs := make(map[string]int64, len(demoUsers)+1)

	var adminID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE username = 'admin' AND deleted_at IS NULL`).Scan(&adminID); err != nil {
		return nil, 0, fmt.Errorf("admin user missing (run core seed first): %w", err)
	}
	userIDs["admin"] = adminID

	pwHash, err := bcrypt.GenerateFromPassword([]byte("Demo@2026"), 10)
	if err != nil {
		return nil, 0, err
	}
	now := time.Now().UTC()

	for _, u := range demoUsers {
		createdAt := now.Add(-time.Duration(u.DaysAgo) * 24 * time.Hour)
		var uid int64
		err := tx.QueryRow(ctx, `
			INSERT INTO users (username, email_norm, password_hash, status, email_verified_at, last_login_at, last_active_at, created_at, updated_at)
			VALUES ($1, lower($2), $3, 'active', $4, now_utc(), now_utc(), $4, now_utc())
			ON CONFLICT (username) DO UPDATE SET updated_at = users.updated_at
			RETURNING id`,
			u.Username, u.Email, string(pwHash), createdAt).Scan(&uid)
		if err != nil {
			return nil, 0, err
		}
		userIDs[u.Username] = uid

		if _, err := tx.Exec(ctx, `
			INSERT INTO user_profiles (user_id, display_name, bio, website, location, profile_visibility, show_followers)
			VALUES ($1, $2, $3, $4, $5, 'public', TRUE)
			ON CONFLICT (user_id) DO NOTHING`,
			uid, u.DisplayName, u.Bio, u.Website, u.Location); err != nil {
			return nil, 0, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role_id, scope_type)
			SELECT $1, id, 'global' FROM roles WHERE code = 'member'
			ON CONFLICT DO NOTHING`, uid); err != nil {
			return nil, 0, err
		}
	}
	return userIDs, adminID, nil
}

func ensureDemoTagsTx(ctx context.Context, tx pgx.Tx) error {
	for _, t := range demoTags {
		if _, err := tx.Exec(ctx, `
			INSERT INTO tags (name, slug, description, status)
			VALUES ($1, $2, $3, 'active')
			ON CONFLICT (slug) DO NOTHING`,
			t.Name, t.Slug, t.Description); err != nil {
			return err
		}
	}
	return nil
}

func ensureDemoFollowsTx(ctx context.Context, tx pgx.Tx, userIDs map[string]int64, adminID int64) error {
	followPairs := [][2]string{
		{"linmo", "chenxi"}, {"linmo", "tangwei"}, {"linmo", "admin"},
		{"chenxi", "linmo"}, {"chenxi", "suxin"}, {"chenxi", "zhouyuan"},
		{"zhouyuan", "suxin"}, {"zhouyuan", "linmo"},
		{"suxin", "chenxi"}, {"suxin", "zhouyuan"}, {"suxin", "admin"},
		{"tangwei", "linmo"}, {"tangwei", "chenxi"}, {"tangwei", "admin"},
	}
	for _, pair := range followPairs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_follows (follower_id, following_id)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			userIDs[pair[0]], userIDs[pair[1]]); err != nil {
			return err
		}
	}
	// 默认版块与标签关注，确保“关注”信息流有丰富内容
	for _, catSlug := range []string{"tech", "general", "life"} {
		for _, uname := range []string{"linmo", "chenxi", "zhouyuan"} {
			if _, err := tx.Exec(ctx, `
				INSERT INTO category_follows (category_id, user_id)
				SELECT c.id, $2 FROM categories c WHERE c.slug = $1
				ON CONFLICT DO NOTHING`, catSlug, userIDs[uname]); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO category_follows (category_id, user_id)
		SELECT id, $1 FROM categories WHERE slug IN ('general', 'tech')
		ON CONFLICT DO NOTHING`, adminID); err != nil {
		return err
	}
	for _, slug := range []string{"golang", "postgresql", "nextjs"} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO tag_follows (tag_id, user_id)
			SELECT id, $2 FROM tags WHERE slug = $1
			ON CONFLICT DO NOTHING`, slug, userIDs["linmo"]); err != nil {
			return err
		}
	}
	return nil
}

func ensureDemoPostsTx(ctx context.Context, tx pgx.Tx, userIDs map[string]int64) (map[string]int64, map[string]bool, map[string]int64, error) {
	postIDs := make(map[string]int64, len(demoPosts))
	newPosts := make(map[string]bool)
	acceptedComments := make(map[string]int64)
	now := time.Now().UTC()

	for _, p := range demoPosts {
		authorID := userIDs[p.Author]
		var catID int64
		if err := tx.QueryRow(ctx, `SELECT id FROM categories WHERE slug = $1`, p.CategorySlug).Scan(&catID); err != nil {
			return nil, nil, nil, fmt.Errorf("category %s: %w", p.CategorySlug, err)
		}

		var postID int64
		var isNew bool
		err := tx.QueryRow(ctx, `
			SELECT id FROM posts
			WHERE author_id = $1 AND title = $2 AND deleted_at IS NULL
			LIMIT 1`, authorID, p.Title).Scan(&postID)
		if errors.Is(err, pgx.ErrNoRows) {
			rendered, rerr := markdown.Render(p.Body)
			if rerr != nil {
				return nil, nil, nil, rerr
			}
			excerpt := markdown.Excerpt(p.Body, 120)
			publishedAt := now.Add(-time.Duration(p.HoursAgo) * time.Hour)

			err = tx.QueryRow(ctx, `
				INSERT INTO posts (
					author_id, category_id, post_type, title, body_markdown, body_rendered, excerpt,
					status, is_pinned, is_featured, view_count, published_at, last_activity_at, created_at, updated_at
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12, $12, $12)
				RETURNING id`,
				authorID, catID, p.PostType, p.Title, p.Body, rendered, excerpt,
				p.Status, p.IsPinned, p.IsFeatured, p.Views, publishedAt).Scan(&postID)
			if err != nil {
				return nil, nil, nil, err
			}
			isNew = true
			newPosts[p.Key] = true
		} else if err != nil {
			return nil, nil, nil, err
		}

		postIDs[p.Key] = postID

		// 标签关联（幂等）
		for _, tagSlug := range p.Tags {
			if _, err := tx.Exec(ctx, `
				INSERT INTO post_tags (post_id, tag_id)
				SELECT $1, id FROM tags WHERE slug = $2
				ON CONFLICT DO NOTHING`, postID, tagSlug); err != nil {
				return nil, nil, nil, err
			}
		}

		// 投票定义与示例票数
		if p.Poll != nil {
			if err := ensurePollForPostTx(ctx, tx, postID, p.Poll, userIDs, now); err != nil {
				return nil, nil, nil, err
			}
		}

		// 评论与楼中楼（仅在首次插入该演示帖或尚无评论时写入，确保可重复执行不翻倍）
		var existingComments int64
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM comments WHERE post_id = $1`, postID).Scan(&existingComments); err != nil {
			return nil, nil, nil, err
		}
		if isNew || existingComments == 0 {
			publishedAt := now.Add(-time.Duration(p.HoursAgo) * time.Hour)
			commentIDMap := make(map[string]int64, len(p.Comments))
			commentRootMap := make(map[string]int64, len(p.Comments))

			for _, cm := range p.Comments {
				rendered, rerr := markdown.Render(cm.Body)
				if rerr != nil {
					return nil, nil, nil, rerr
				}
				var parentID, rootID, quoteID *int64
				if cm.ParentKey != "" {
					pid := commentIDMap[cm.ParentKey]
					rid := commentRootMap[cm.ParentKey]
					parentID = &pid
					rootID = &rid
				}
				if cm.QuoteKey != "" {
					qid := commentIDMap[cm.QuoteKey]
					quoteID = &qid
				}
				cmAt := publishedAt.Add(time.Duration(cm.HoursAfterPost) * time.Hour)
				if cmAt.After(now) {
					cmAt = now.Add(-10 * time.Minute)
				}

				var cid int64
				if err := tx.QueryRow(ctx, `
					INSERT INTO comments (
						post_id, author_id, parent_id, root_id, depth,
						body_markdown, body_rendered, quote_comment_id, status,
						is_accepted_answer, created_at, updated_at
					)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'published', $9, $10, $10)
					RETURNING id`,
					postID, userIDs[cm.Author], parentID, rootID, cm.Depth,
					cm.Body, rendered, quoteID, cm.Accepted, cmAt).Scan(&cid); err != nil {
					return nil, nil, nil, err
				}
				commentIDMap[cm.Key] = cid
				if cm.ParentKey == "" {
					commentRootMap[cm.Key] = cid
				} else {
					commentRootMap[cm.Key] = commentRootMap[cm.ParentKey]
				}
				if cm.Accepted {
					acceptedComments[cm.Author] = cid
				}

				for _, liker := range cm.LikedBy {
					if _, err := tx.Exec(ctx, `
						INSERT INTO comment_likes (user_id, comment_id, created_at)
						VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
						userIDs[liker], cid, cmAt.Add(15*time.Minute)); err != nil {
						return nil, nil, nil, err
					}
				}
			}
		}

		// 帖子点赞与收藏（幂等）
		for _, liker := range p.LikedBy {
			if _, err := tx.Exec(ctx, `
				INSERT INTO post_likes (user_id, post_id)
				VALUES ($1, $2) ON CONFLICT DO NOTHING`, userIDs[liker], postID); err != nil {
				return nil, nil, nil, err
			}
		}
		for _, saver := range p.BookmarkedBy {
			if _, err := tx.Exec(ctx, `
				INSERT INTO bookmarks (user_id, post_id)
				VALUES ($1, $2) ON CONFLICT (user_id, post_id) DO NOTHING`, userIDs[saver], postID); err != nil {
				return nil, nil, nil, err
			}
		}
	}

	return postIDs, newPosts, acceptedComments, nil
}

func ensurePollForPostTx(ctx context.Context, tx pgx.Tx, postID int64, spec *demoPollSpec, userIDs map[string]int64, now time.Time) error {
	var pollID int64
	err := tx.QueryRow(ctx, `SELECT id FROM polls WHERE post_id = $1`, postID).Scan(&pollID)
	if errors.Is(err, pgx.ErrNoRows) {
		endsAt := now.Add(time.Duration(spec.EndsInDays) * 24 * time.Hour)
		if err := tx.QueryRow(ctx, `
			INSERT INTO polls (post_id, multiple_choice, max_choices, allow_revote, ends_at)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			postID, spec.MultipleChoice, spec.MaxChoices, spec.AllowRevote, endsAt).Scan(&pollID); err != nil {
			return err
		}
		optIDs := make([]int64, len(spec.Options))
		for i, text := range spec.Options {
			if err := tx.QueryRow(ctx, `
				INSERT INTO poll_options (poll_id, content, sort_order)
				VALUES ($1, $2, $3) RETURNING id`,
				pollID, text, i).Scan(&optIDs[i]); err != nil {
				return err
			}
		}
		for uname, idxs := range spec.Votes {
			uid := userIDs[uname]
			for _, idx := range idxs {
				if idx < 0 || idx >= len(optIDs) {
					continue
				}
				if _, err := tx.Exec(ctx, `
					INSERT INTO poll_votes (poll_id, option_id, user_id)
					VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
					pollID, optIDs[idx], uid); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return err
}

func ensureDemoActivitiesAndAnnouncementsTx(ctx context.Context, tx pgx.Tx, userIDs map[string]int64, adminID int64) error {
	now := time.Now().UTC()
	for _, a := range demoActivities {
		var actID int64
		err := tx.QueryRow(ctx, `SELECT id FROM activities WHERE title = $1 LIMIT 1`, a.Title).Scan(&actID)
		if errors.Is(err, pgx.ErrNoRows) {
			startsAt := now.Add(time.Duration(a.StartsInDays) * 24 * time.Hour)
			endsAt := startsAt.Add(time.Duration(a.DurationHrs) * time.Hour)
			signupStarts := now.Add(-24 * time.Hour)
			signupEnds := startsAt
			if err := tx.QueryRow(ctx, `
				INSERT INTO activities (
					title, description, location, starts_at, ends_at,
					signup_starts_at, signup_ends_at, capacity, status, reward_points, created_by
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'published', $9, $10)
				RETURNING id`,
				a.Title, a.Description, a.Location, startsAt, endsAt,
				signupStarts, signupEnds, a.Capacity, a.RewardPoints, userIDs[a.Creator]).Scan(&actID); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		for _, uname := range a.Participants {
			if _, err := tx.Exec(ctx, `
				INSERT INTO activity_participants (activity_id, user_id, status)
				VALUES ($1, $2, 'signed_up')
				ON CONFLICT (activity_id, user_id) DO NOTHING`,
				actID, userIDs[uname]); err != nil {
				return err
			}
		}
	}

	const annTitle = "第一期线上技术沙龙与主题投票已开启"
	var annID int64
	err := tx.QueryRow(ctx, `SELECT id FROM announcements WHERE title = $1 LIMIT 1`, annTitle).Scan(&annID)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO announcements (title, body, status, author_id, published_at)
			VALUES ($1, $2, 'published', $3, now_utc() - interval '2 hours')`,
			annTitle,
			"欢迎前往「社区活动」报名第一期线上技术沙龙，或在综合讨论参与分享会主题投票。",
			adminID); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func ensureDemoPointsAndBadgesTx(ctx context.Context, tx pgx.Tx, userIDs map[string]int64, adminID int64, postIDs map[string]int64, acceptedComments map[string]int64) error {
	now := time.Now().UTC()

	for _, u := range demoUsers {
		uid := userIDs[u.Username]
		if err := grantDemoPointsTx(ctx, tx, uid, "registration", uid, "demo:reg:"+u.Username, 20, "完成注册并验证邮箱", 0); err != nil {
			return err
		}
		for i := 0; i < u.StreakDays; i++ {
			day := now.AddDate(0, 0, -i).Format("2006-01-02")
			if _, err := tx.Exec(ctx, `
				INSERT INTO sign_ins (user_id, check_date, points)
				VALUES ($1, $2::date, 5) ON CONFLICT DO NOTHING`,
				uid, day); err != nil {
				return err
			}
			if err := grantDemoPointsTx(ctx, tx, uid, "sign_in", 0, "signin:"+fmt.Sprintf("%d", uid)+":"+day, 5, "每日签到", 0); err != nil {
				return err
			}
		}
		for _, badgeCode := range u.Badges {
			if _, err := tx.Exec(ctx, `
				INSERT INTO user_badges (user_id, badge_id, awarded_by)
				SELECT $1, id, NULLIF($2, 0) FROM badges WHERE code = $3 AND is_enabled = TRUE
				ON CONFLICT (user_id, badge_id) DO NOTHING`,
				uid, adminID, badgeCode); err != nil {
				return err
			}
		}
	}

	for _, p := range demoPosts {
		pid := postIDs[p.Key]
		uid := userIDs[p.Author]
		if err := grantDemoPointsTx(ctx, tx, uid, "post_created", pid, fmt.Sprintf("post_created:%d", pid), 10, "发布内容奖励", 0); err != nil {
			return err
		}
		if len(p.LikedBy) > 0 {
			if err := grantDemoPointsTx(ctx, tx, uid, "post_liked", pid, fmt.Sprintf("like_post:%d", pid), 1, "帖子获得首个点赞", 0); err != nil {
				return err
			}
		}
	}

	for uname, cid := range acceptedComments {
		if err := grantDemoPointsTx(ctx, tx, userIDs[uname], "answer_accepted", cid, fmt.Sprintf("accept:%d", cid), 20, "回答被采纳为最佳答案", 0); err != nil {
			return err
		}
	}
	return nil
}

func grantDemoPointsTx(ctx context.Context, tx pgx.Tx, userID int64, sourceType string, sourceID int64, key string, amount int64, note string, operatorID int64) error {
	var txID int64
	err := tx.QueryRow(ctx, `
		INSERT INTO point_transactions (user_id, amount, balance_after, source_type, source_id, idempotency_key, note, operator_id)
		VALUES ($1, $2, 0, $3, $4, $5, $6, NULLIF($7, 0))
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id`,
		userID, amount, sourceType, sourceID, key, note, operatorID).Scan(&txID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var newBalance int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO point_accounts (user_id, balance, total_earned)
		VALUES ($1, GREATEST(0, $2), GREATEST(0, $2))
		ON CONFLICT (user_id) DO UPDATE SET
			balance = GREATEST(0, point_accounts.balance + $2),
			total_earned = point_accounts.total_earned + GREATEST(0, $2),
			updated_at = now_utc()
		RETURNING balance`,
		userID, amount).Scan(&newBalance); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE point_transactions SET balance_after = $2 WHERE id = $1`, txID, newBalance)
	return err
}

func recountDemoMetricsTx(ctx context.Context, tx pgx.Tx, postIDs map[string]int64) error {
	for _, pid := range postIDs {
		if _, err := tx.Exec(ctx, `
			UPDATE posts p SET
				like_count = (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id),
				bookmark_count = (SELECT COUNT(*) FROM bookmarks b WHERE b.post_id = p.id),
				comment_count = (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id AND c.status = 'published' AND c.deleted_at IS NULL),
				last_activity_at = GREATEST(
					COALESCE(p.published_at, p.created_at),
					COALESCE((SELECT MAX(c.created_at) FROM comments c WHERE c.post_id = p.id AND c.status = 'published' AND c.deleted_at IS NULL), COALESCE(p.published_at, p.created_at))
				)
			WHERE p.id = $1`, pid); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE comments c SET
				like_count = (SELECT COUNT(*) FROM comment_likes cl WHERE cl.comment_id = c.id),
				reply_count = (SELECT COUNT(*) FROM comments r WHERE r.parent_id = c.id AND r.status = 'published' AND r.deleted_at IS NULL)
			WHERE c.post_id = $1`, pid); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE categories c SET post_count = (
			SELECT COUNT(*) FROM posts p
			WHERE p.category_id = c.id AND p.status IN ('published', 'locked') AND p.deleted_at IS NULL
		)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE tags t SET post_count = (
			SELECT COUNT(*) FROM post_tags pt
			JOIN posts p ON p.id = pt.post_id
			WHERE pt.tag_id = t.id AND p.status IN ('published', 'locked') AND p.deleted_at IS NULL
		)`); err != nil {
		return err
	}
	return nil
}
