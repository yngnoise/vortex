DROP TRIGGER IF EXISTS trg_conversations_updated_at ON conversations;
DROP TABLE IF EXISTS message_reactions CASCADE;
DROP TABLE IF EXISTS attachments CASCADE;
DROP TABLE IF EXISTS messages CASCADE;
DROP TABLE IF EXISTS conversation_members CASCADE;
DROP TABLE IF EXISTS conversations CASCADE;
