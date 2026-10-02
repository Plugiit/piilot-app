-- name: EnqueueEmail :exec
INSERT INTO email_outbox (kind, to_address, subject, text_body, html_body)
VALUES ($1, $2, $3, $4, $5);

-- name: ClaimDueEmail :one
-- Le prochain e-mail a envoyer, verrouille pour la transaction de l'envoi.
SELECT * FROM email_outbox
WHERE status = 'pending' AND next_attempt_at <= now()
ORDER BY next_attempt_at
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- name: MarkEmailSent :exec
-- Les corps sont effaces : ils portent des liens a usage unique, qui n'ont
-- plus rien a faire en base une fois l'e-mail parti.
UPDATE email_outbox
SET status = 'sent', sent_at = now(), attempts = attempts + 1,
    text_body = '', html_body = '', last_error = ''
WHERE id = $1;

-- name: MarkEmailRetry :exec
UPDATE email_outbox
SET attempts = attempts + 1, next_attempt_at = $2, last_error = $3
WHERE id = $1;

-- name: MarkEmailFailed :exec
UPDATE email_outbox
SET status = 'failed', attempts = attempts + 1, last_error = $2,
    text_body = '', html_body = ''
WHERE id = $1;
