DROP TRIGGER IF EXISTS deliverable_versions_client_activity ON deliverable_versions;
DROP TRIGGER IF EXISTS tickets_client_stats ON tickets;
DROP FUNCTION IF EXISTS deliverable_versions_client_activity();
DROP FUNCTION IF EXISTS tickets_client_stats();
DROP FUNCTION IF EXISTS bump_client_activity(uuid, timestamptz, integer, integer);
DROP FUNCTION IF EXISTS refresh_client_ticket_stats(uuid);
DROP TABLE IF EXISTS client_activity_months;
DROP TABLE IF EXISTS client_ticket_stats;
