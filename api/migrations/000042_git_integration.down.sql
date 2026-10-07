DELETE FROM client_interactions WHERE kind = 'deployment';
ALTER TABLE client_interactions DROP CONSTRAINT client_interactions_kind_check;
ALTER TABLE client_interactions
    ADD CONSTRAINT client_interactions_kind_check CHECK (kind IN (
        'note', 'call', 'meeting', 'email',
        'project_created', 'deliverable_validated', 'ticket_opened'
    ));
DROP TABLE IF EXISTS deployments;
DROP TABLE IF EXISTS pull_request_links;
DROP TABLE IF EXISTS pull_requests;
DROP TABLE IF EXISTS git_webhook;
ALTER TABLE tasks DROP COLUMN IF EXISTS numero;
DROP INDEX IF EXISTS projects_repo_url_idx;
ALTER TABLE projects DROP COLUMN IF EXISTS repo_url;
