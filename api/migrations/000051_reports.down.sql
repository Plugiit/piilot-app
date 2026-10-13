DROP TABLE IF EXISTS project_report_opt_outs;
DROP TABLE IF EXISTS project_report_recipients;
DROP TRIGGER IF EXISTS reports_guard ON reports;
DROP FUNCTION IF EXISTS reports_guard();
DROP TABLE IF EXISTS reports;
DROP TABLE IF EXISTS report_settings;
