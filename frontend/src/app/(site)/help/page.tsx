"use client";

// 帮助与规则页。
import Link from "next/link";
import { PageHeader } from "@/components/ui/States";

export default function HelpPage() {
  return (
    <div style={{ maxWidth: 740, margin: "0 auto" }}>
      <PageHeader title="帮助与社区规则" subtitle="共同维护理性、友善、有沉淀的交流环境" />
      <article className="card prose-community" style={{ padding: "24px 28px" }}>
        <h2 style={{ marginTop: 0 }}>社区公约</h2>
        <ol>
          <li>尊重每一位成员：禁止人身攻击、骚扰与恶意引战。</li>
          <li>禁止垃圾广告与恶意刷屏，禁止发布违法违规内容。</li>
          <li>引用他人观点或资料请注明来源，尊重原创与知识产权。</li>
          <li>发布内容前请选择恰当的版块与标签，对标题和正文负责。</li>
          <li>如发现违规内容可使用「举报」功能，管理团队将及时核查处理。</li>
        </ol>

        <h2>常见问题</h2>
        <h3>为什么暂时无法发帖或评论？</h3>
        <p>
          新注册账号需完成<b>邮箱验证</b>后才能发表帖子和评论，请前往{" "}
          <Link href="/settings">账号设置</Link> 查收或重发验证邮件；部分特定版块可能设置额外发帖权限。
        </p>
        <h3>如何获得积分、等级和勋章？</h3>
        <p>
          通过注册、每日签到、发表帖子、获得赞同、回答被采纳或出席社区活动均可累积积分与等级成长。前往{" "}
          <Link href="/me/points">积分中心</Link> 即可签到并查看全部勋章解锁条件。
        </p>
        <h3>如何使用问答、投票、活动与私信？</h3>
        <p>
          在 <Link href="/compose">发布页</Link> 可切换「问答帖」或「投票帖」，提问者可在评论区采纳最佳回答；在{" "}
          <Link href="/activities">社区活动</Link> 可报名活动；在成员主页点击「发私信」可发起一对一会话，并可在{" "}
          <Link href="/settings">隐私设置</Link> 中随时管理私信接收开关。
        </p>
        <h3>如何修改密码？</h3>
        <p>在 <Link href="/settings">账号设置 → 修改密码</Link> 通过邮箱验证重置，重置后所有设备将自动下线。</p>
        <h3>内容被处理或处罚有异议怎么办？</h3>
        <p>
          版主与管理员的操作均会通过通知告知原因。如有异议，请前往{" "}
          <Link href="/me/appeals">申诉中心</Link> 提交复核申诉。
        </p>

        <h2 id="roadmap">后续规划</h2>
        <p>当前论坛讨论、问答投票、活动积分与私信功能均已上线，后续按需推进：</p>
        <ul>
          <li>知识库整理与检索增强能力</li>
          <li>人机校验与账号自助注销流程</li>
          <li>自定义角色权限体系扩展</li>
        </ul>
      </article>
    </div>
  );
}
