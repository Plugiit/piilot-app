DROP TABLE bot_event_cursor;
DROP TABLE bot_events;
DROP TABLE discord_link_codes;
DROP TABLE discord_links;
DROP TABLE bot_keys;

DELETE FROM permissions WHERE code IN ('discord.link', 'discord.read', 'discord.redeem', 'events.read');

ALTER TABLE invitations DROP CONSTRAINT invitations_role_not_bot;
ALTER TABLE users DROP CONSTRAINT users_role_not_bot;

DELETE FROM roles WHERE code = 'bot';
