DELETE FROM notifications WHERE kind LIKE 'ticket_%' OR kind LIKE 'deliverable_%';
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_kind_check CHECK (kind IN (
        'task_created',
        'task_status_changed',
        'task_assigned',
        'task_unassigned',
        'task_commented',
        'task_due_changed',
        'project_created'
    ));
ALTER TABLE notifications DROP COLUMN IF EXISTS deliverable_id, DROP COLUMN IF EXISTS ticket_id;

UPDATE permissions SET label = 'Consulter le temps passé' WHERE code = 'time.read';
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'team' AND p.code IN ('time.read', 'roles.read')
ON CONFLICT DO NOTHING;

DELETE FROM permissions WHERE code IN ('dashboard.read', 'budgets.read', 'pipeline.read');
