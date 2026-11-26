ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_not_empty;
ALTER TABLE messages ADD CONSTRAINT messages_not_empty
    CHECK (char_length(body_text) > 0);
