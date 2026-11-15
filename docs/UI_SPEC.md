# UI_SPEC — 前端 UI 规范

## 1. 设计定位

前台 = 现代内容社区（内容优先、阅读舒适、层级清晰、克制装饰）；后台 = 成熟企业级管理风格（Ant Design）。两套体系共享品牌色与语义色变量。

## 2. Design Tokens（`src/styles/tokens.css`）

```css
:root {
  /* 中性色 */
  --bg-page:#f7f8fa; --bg-card:#ffffff; --bg-hover:#f0f2f5;
  --border:#e5e7eb; --border-strong:#d1d5db;
  --text-1:#111827; --text-2:#4b5563; --text-3:#9ca3af;
  /* 品牌与语义色 */
  --brand:#4f46e5; --brand-hover:#4338ca; --brand-soft:#eef2ff;
  --success:#16a34a; --warning:#d97706; --danger:#dc2626; --info:#2563eb;
  /* 圆角/阴影/间距 */
  --radius-sm:6px; --radius:10px; --radius-lg:14px;
  --shadow-card:0 1px 2px rgba(16,24,40,.05);
  --shadow-pop:0 8px 24px rgba(16,24,40,.12);
  --space-1:4px; --space-2:8px; --space-3:12px; --space-4:16px; --space-6:24px;
  --content-max:1120px; --reading-max:720px;
}
[data-theme="dark"] { /* 全量映射暗色 */ }
```

- 品牌色仅用于主操作/链接/选中态；success/warning/danger/info 用于语义标签与图表，禁止同色滥用。
- 卡片：`bg-card + 1px border + shadow-card + radius`；阴影只用于浮层。

## 3. 布局

- 桌面 ≥1024px：顶部导航（logo / 首页 / 版块 / 搜索框 / 发布按钮 / 通知铃铛(未读角标) / 头像菜单）+ 内容主栏（约 720px）+ 右侧栏（公告、热门标签、活跃版块、推荐用户）。
- 平板 640–1024：侧栏折叠到主栏下方。
- 手机 <640：紧凑顶栏 + 底部导航（首页/版块/发布/消息/我的）；正文 16px、行高 1.75、点击区 ≥40px。
- 正文阅读宽度 ≤ `--reading-max`；全文宽度 ≤ `--content-max` 居中。

## 4. 组件规范（components/ui）

| 组件 | 要点 |
|---|---|
| Button | primary/secondary/text/danger；hover/active/focus-visible/disabled/loading 完整 |
| Card | 细边框轻阴影统一圆角 |
| Avatar | 尺寸 24/32/40；默认生成首字母头像 |
| Tag | 版块/标签/状态语义色 |
| Tabs | 首页流切换，底部 2px 指示条，`role=tablist` |
| Skeleton | 列表/详情加载态（卡片骨架屏） |
| EmptyState | 图标 + 说明 + 行动按钮（如「去发帖」） |
| ErrorState | 可重试按钮 |
| Toast/Dialog | 操作成功/失败反馈；危险操作确认弹窗 |
| Pagination | page/page_size 控件 |

## 5. 页面状态矩阵（强制）

每个数据页面必须实现：**loading**（骨架屏）、**empty**（引导动作）、**error**（重试）、**success**；表单实现：提交 loading、字段级错误、防重复提交、成功反馈。

## 6. 关键页面

- **首页**：Tab（推荐/最新/热门/精华/关注）+ 帖子卡片流 + 右侧栏；未登录点「关注」→ 引导登录卡片。
- **帖子卡片**：标题（semibold 17px）→ 摘要 2 行截断 → 元信息行（头像 24px、昵称、相对时间、版块 Tag、标签、👍 评论数）。
- **帖子详情**：面包屑(版块) → 标题 → 元信息 → 净化 Markdown 正文（代码块高亮、图片懒加载）→ 操作条（点赞/收藏/分享/举报）→ 评论树（深度≤3 缩进 + 「引用回复」标注）。
- **编辑器**：textarea + 工具栏（加粗/斜体/代码/链接/图片/标题/引用）+ 预览切换 + 标签选择（最多5）+ 版块选择 + 草稿自动保存（zustand 持久 + API）。
- **后台（Ant Design）**：`/admin` 独立布局（侧边菜单 + 面包屑 + 内容区）；统一 ProTable 风格表格、筛选区、详情 Drawer、批量操作、状态 Tag 语义色。
- **reduced motion**：`@media (prefers-reduced-motion: reduce)` 关闭过渡动画。

## 7. 可访问性

焦点可见（`:focus-visible` 2px brand 描边）、语义化标签与 aria-label、对比度 ≥ 4.5:1、键盘可操作（Tab 顺序、Esc 关闭浮层）。

## 8. 前端质量红线

- 所有按钮/表单/菜单必须真实调用 API；未实现功能入口标记「规划中」徽标或置灰提示。
- 不在 localStorage 存长期凭证（Cookie 由后端设置）；Zustand 持久化仅存 UI 偏好与草稿文本。
- SSR 页面不访问 window/localStorage；fetch 带 `credentials:'include'`。
- 路由保护：`/me/*`、`/admin/*` 客户端守卫 + 服务端权限兜底。
