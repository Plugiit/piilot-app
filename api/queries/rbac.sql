-- name: RoleHasPermission :one
-- Question posee par la garde a chaque requete protegee. Les deux jointures
-- portent sur des cles primaires et des index uniques : c'est une lecture
-- indexee, pas un balayage.
SELECT EXISTS (
    SELECT 1
    FROM role_permissions rp
    JOIN roles r       ON r.id = rp.role_id
    JOIN permissions p ON p.id = rp.permission_id
    WHERE r.code = sqlc.arg('role_code') AND p.code = sqlc.arg('permission_code')
) AS granted;

-- name: ListPermissionsByRole :many
-- Permissions d'un role, servies telles quelles au front pour qu'il masque les
-- actions inaccessibles. Le front cache des boutons, il ne protege rien : la
-- garde reste la seule autorite.
SELECT p.code
FROM role_permissions rp
JOIN roles r       ON r.id = rp.role_id
JOIN permissions p ON p.id = rp.permission_id
WHERE r.code = sqlc.arg('role_code')
ORDER BY p.code
LIMIT 500;

-- name: ListRoles :many
SELECT * FROM roles
ORDER BY code
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: GetRoleByCode :one
SELECT * FROM roles
WHERE code = $1;

-- name: ListRolePermissionCodes :many
-- Matrice de l'ecran « Roles » : chaque role et ses permissions, en une
-- requete. Bornee par construction : quelques roles, quelques dizaines de
-- permissions.
SELECT r.code AS role_code, p.code AS permission_code
FROM role_permissions rp
JOIN roles r       ON r.id = rp.role_id
JOIN permissions p ON p.id = rp.permission_id
ORDER BY r.code, p.code
LIMIT 1000;

-- name: ListPermissions :many
SELECT * FROM permissions ORDER BY code LIMIT 500;

-- name: CountUsersByRole :many
SELECT role, count(*) AS total FROM users
WHERE deleted_at IS NULL AND disabled_at IS NULL
GROUP BY role;

-- name: ClearRolePermissions :exec
DELETE FROM role_permissions
WHERE role_id = (SELECT id FROM roles WHERE code = $1);

-- name: GrantRolePermission :exec
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.code = sqlc.arg('role_code') AND p.code = sqlc.arg('permission_code')
ON CONFLICT DO NOTHING;
