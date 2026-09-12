DROP TRIGGER IF EXISTS attachments_task_count ON attachments;
DROP TRIGGER IF EXISTS task_comments_count ON task_comments;
DROP TRIGGER IF EXISTS subtasks_task_counts ON subtasks;

DROP FUNCTION IF EXISTS attachments_touch_task_count();
DROP FUNCTION IF EXISTS refresh_task_attachment_count(uuid);
DROP FUNCTION IF EXISTS task_comments_touch_count();
DROP FUNCTION IF EXISTS refresh_task_comment_count(uuid);
DROP FUNCTION IF EXISTS subtasks_touch_task_counts();
DROP FUNCTION IF EXISTS refresh_task_subtask_counts(uuid);

ALTER TABLE tasks
    DROP COLUMN IF EXISTS attachments_count,
    DROP COLUMN IF EXISTS comments_count,
    DROP COLUMN IF EXISTS subtasks_done,
    DROP COLUMN IF EXISTS subtasks_total;

-- Les pieces jointes de tache disparaissent avec la colonne qui les portait.
DELETE FROM attachments WHERE task_id IS NOT NULL;

DROP INDEX IF EXISTS attachments_task_idx;

ALTER TABLE attachments
    DROP CONSTRAINT IF EXISTS attachments_owner;

ALTER TABLE attachments
    DROP COLUMN IF EXISTS task_id,
    ALTER COLUMN project_id SET NOT NULL;

ALTER INDEX attachments_project_idx RENAME TO project_files_project_idx;
ALTER TABLE attachments RENAME TO project_files;
