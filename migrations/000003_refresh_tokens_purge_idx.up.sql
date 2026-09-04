-- La purge periodique filtre sur expires_at. Sans cet index, elle balaye toute
-- la table a chaque passage — supportable sur une table jeune, plus du tout une
-- fois que chaque connexion et chaque rotation y ont laisse une ligne.
CREATE INDEX refresh_tokens_expires_idx ON refresh_tokens (expires_at);
