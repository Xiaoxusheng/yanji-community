-- 000011：允许已撤回私信（status = 'recalled'）清空 body_text。
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_not_empty;
ALTER TABLE messages ADD CONSTRAINT messages_not_empty
    CHECK (status = 'recalled' OR char_length(body_text) > 0);
