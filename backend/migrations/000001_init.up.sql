-- 000001：基础扩展与公共函数。
-- 时间统一 UTC。
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE OR REPLACE FUNCTION now_utc() RETURNS timestamptz AS $$
  SELECT timezone('utc', now());
$$ LANGUAGE sql STABLE;
