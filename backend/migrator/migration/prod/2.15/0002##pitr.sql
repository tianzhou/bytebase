ALTER TABLE db DROP COLUMN source_backup_id;
DROP TABLE backup;
DROP TABLE backup_setting;
DELETE FROM anomaly WHERE type = 'bb.anomaly.database.backup.missing';
DELETE FROM anomaly WHERE type = 'bb.anomaly.database.backup.policy-violation';
DELETE FROM policy WHERE type = 'bb.policy.backup-plan';
DELETE FROM policy WHERE type = 'bb.policy.workspace-iam';
DELETE FROM plan_check_run WHERE type = 'bb.plan-check.database.pitr.mysql';
DELETE FROM task_dag;
DELETE FROM external_approval; 
DELETE FROM task_run WHERE task_id IN (SELECT id FROM task WHERE type = 'bb.task.database.backup' OR type = 'bb.task.database.restore.pitr.restore' OR type = 'bb.task.database.restore.pitr.cutover');
DELETE FROM task WHERE type = 'bb.task.database.backup' OR type = 'bb.task.database.restore.pitr.restore' OR type = 'bb.task.database.restore.pitr.cutover';
DELETE FROM stage WHERE NOT EXISTS (SELECT task.id FROM task WHERE task.stage_id = stage.id);
DELETE FROM instance_change_history WHERE issue_id IN (SELECT id FROM issue WHERE NOT EXISTS (SELECT task.id FROM task WHERE task.pipeline_id = issue.pipeline_id) AND issue.type = 'bb.issue.database.general');
DELETE FROM instance_change_history WHERE type = 'BRANCH';
DELETE FROM issue WHERE NOT EXISTS (SELECT task.id FROM task WHERE task.pipeline_id = issue.pipeline_id) AND issue.type = 'bb.issue.database.general';
DELETE FROM plan WHERE pipeline_id IN (SELECT id FROM pipeline WHERE NOT EXISTS (SELECT task.id FROM task WHERE task.pipeline_id = pipeline.id));
DELETE FROM pipeline WHERE NOT EXISTS (SELECT task.id FROM task WHERE task.pipeline_id = pipeline.id);
