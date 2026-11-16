-- 000010：中文搜索兜底——正文 trigram 索引（标题 trgm 见 000005）。
CREATE INDEX idx_posts_body_trgm ON posts USING GIN (body_markdown gin_trgm_ops);
