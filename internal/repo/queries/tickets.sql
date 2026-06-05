-- name: CreateTicket :one
INSERT INTO tickets (tenant_id, customer_id, type, subject, description, status, priority, assigned_user_id, created_by)
VALUES ($1, $2, $3, $4, $5, 'open', $6, $7, $8)
RETURNING *;

-- name: GetTicket :one
SELECT * FROM tickets WHERE id = $1 AND tenant_id = $2;

-- name: ListTickets :many
SELECT * FROM tickets WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: UpdateTicketStatus :exec
UPDATE tickets SET
    status = $3,
    resolved_at = CASE WHEN $3 = 'resolved' THEN now() ELSE resolved_at END,
    updated_at = now()
WHERE id = $1 AND tenant_id = $2;

-- name: AssignTicket :exec
UPDATE tickets SET assigned_user_id = $3, updated_at = now()
WHERE id = $1 AND tenant_id = $2;

-- name: AddTicketEvent :one
INSERT INTO ticket_events (ticket_id, user_id, note, status_from, status_to)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListTicketEvents :many
SELECT * FROM ticket_events WHERE ticket_id = $1 ORDER BY created_at;

-- name: ListTicketsByCustomer :many
SELECT * FROM tickets WHERE tenant_id = $1 AND customer_id = $2
ORDER BY created_at DESC LIMIT $3 OFFSET $4;
