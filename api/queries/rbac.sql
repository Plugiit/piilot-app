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
