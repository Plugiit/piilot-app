DROP TABLE oauth_tokens;
DROP TABLE oauth_codes;
DROP TABLE oauth_grants;
DROP TABLE oauth_requests;

DELETE FROM permissions WHERE code = 'claude.connect';
