-- SSH Credentials
-- name: CreateSSHCredential :one
INSERT INTO ssh_credentials (id, workspace_id, name, private_key_enc, fingerprint, username, port)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, workspace_id, name, fingerprint, username, port, created_at, updated_at;

-- name: GetSSHCredential :one
SELECT * FROM ssh_credentials WHERE id = $1 AND workspace_id = $2;

-- name: ListSSHCredentialsByWorkspace :many
SELECT id, workspace_id, name, fingerprint, username, port, created_at, updated_at
FROM ssh_credentials WHERE workspace_id = $1 ORDER BY name;

-- name: DeleteSSHCredential :execrows
DELETE FROM ssh_credentials WHERE id = $1 AND workspace_id = $2;

-- SSM Profiles
-- name: CreateSSMProfile :one
INSERT INTO ssm_profiles (id, workspace_id, name, region, role_arn)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSSMProfile :one
SELECT * FROM ssm_profiles WHERE id = $1 AND workspace_id = $2;

-- name: ListSSMProfilesByWorkspace :many
SELECT * FROM ssm_profiles WHERE workspace_id = $1 ORDER BY name;

-- name: DeleteSSMProfile :execrows
DELETE FROM ssm_profiles WHERE id = $1 AND workspace_id = $2;

-- K8s Clusters
-- name: CreateK8sCluster :one
INSERT INTO k8s_clusters (id, workspace_id, name, kubeconfig_enc, default_namespace)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, workspace_id, name, default_namespace, created_at, updated_at;

-- name: GetK8sCluster :one
SELECT * FROM k8s_clusters WHERE id = $1 AND workspace_id = $2;

-- name: ListK8sClustersByWorkspace :many
SELECT id, workspace_id, name, default_namespace, created_at, updated_at
FROM k8s_clusters WHERE workspace_id = $1 ORDER BY name;

-- name: DeleteK8sCluster :execrows
DELETE FROM k8s_clusters WHERE id = $1 AND workspace_id = $2;
