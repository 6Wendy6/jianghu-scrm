#!/usr/bin/env node
import { spawnSync } from "node:child_process";

function usage() {
  console.log(`usage:
  node scripts/e2e-cleanup.mjs --test-run-id=e2e_YYYYMMDD_HHMMSS --dry-run
  node scripts/e2e-cleanup.mjs --test-run-id=e2e_YYYYMMDD_HHMMSS --apply

Required:
  --test-run-id=<id>     Unique E2E run id. Must start with e2e_.

Options:
  --database-url=<dsn>   PostgreSQL DSN. Defaults to SCRM_TEST_DATABASE_URL only.
  --dry-run              Print counts only. This is the default.
  --apply                Delete only rows matching this test_run_id.
  --allow-shared-db      Allow a DB name that does not include test/e2e/ci.

Safety:
  - Refuses DATABASE_URL fallback.
  - Refuses port 15432.
  - Refuses DELETE without a test_run_id marker.
  - Never runs TRUNCATE, DROP, or unqualified DELETE.
`);
}

function parseArgs(argv) {
  const args = { mode: "dry-run" };
  for (const item of argv) {
    if (item === "--help" || item === "-h") args.help = true;
    else if (item === "--dry-run") args.mode = "dry-run";
    else if (item === "--apply") args.mode = "apply";
    else if (item === "--allow-shared-db") args.allowSharedDB = true;
    else if (item.startsWith("--test-run-id=")) args.testRunID = item.slice("--test-run-id=".length);
    else if (item.startsWith("--database-url=")) args.databaseURL = item.slice("--database-url=".length);
    else throw new Error(`unknown argument: ${item}`);
  }
  return args;
}

function assertSafeInput(args) {
  if (args.help) return;
  if (!args.testRunID || !/^e2e_[A-Za-z0-9_:-]+$/.test(args.testRunID)) {
    throw new Error("missing or invalid --test-run-id. Use a unique id starting with e2e_.");
  }
  const databaseURL = args.databaseURL || process.env.SCRM_TEST_DATABASE_URL || "";
  if (!databaseURL) {
    throw new Error("SCRM_TEST_DATABASE_URL is required. This script never falls back to DATABASE_URL.");
  }
  let parsed;
  try {
    parsed = new URL(databaseURL);
  } catch (err) {
    throw new Error(`invalid PostgreSQL URL: ${err.message}`);
  }
  if (!/^postgres(ql)?:$/.test(parsed.protocol)) {
    throw new Error("database URL must use postgres:// or postgresql://");
  }
  if (parsed.port === "15432") {
    throw new Error("refusing to connect to port 15432; use a temporary DB or SCRM_TEST_DATABASE_URL on a separate port.");
  }
  const dbName = parsed.pathname.replace(/^\//, "");
  if (!args.allowSharedDB && !/(test|e2e|ci)/i.test(dbName)) {
    throw new Error(`refusing database ${dbName}; name must include test/e2e/ci or pass --allow-shared-db.`);
  }
  return databaseURL;
}

function sqlString(value) {
  return `'${String(value).replace(/'/g, "''")}'`;
}

function dbURLForDocker(databaseURL) {
  const parsed = new URL(databaseURL);
  if (["127.0.0.1", "localhost", "::1"].includes(parsed.hostname)) {
    parsed.hostname = "host.docker.internal";
  }
  return parsed.toString();
}

function runPSQL(databaseURL, sql) {
  const psqlArgs = ["--set", "ON_ERROR_STOP=1", "--no-psqlrc", "--quiet", "--tuples-only", "--no-align", databaseURL];
  const local = spawnSync("psql", psqlArgs, {
    input: sql,
    encoding: "utf8",
  });
  if (local.status === 0) return local.stdout.trim();
  if (local.error && local.error.code !== "ENOENT") {
    throw new Error(local.stderr || local.error.message);
  }

  const dockerURL = dbURLForDocker(databaseURL);
  const docker = spawnSync(
    "docker",
    ["run", "--rm", "-i", "postgres:16-alpine", "psql", "--set", "ON_ERROR_STOP=1", "--no-psqlrc", "--quiet", "--tuples-only", "--no-align", dockerURL],
    { input: sql, encoding: "utf8" },
  );
  if (docker.status !== 0) {
    throw new Error(docker.stderr || docker.stdout || "psql execution failed");
  }
  return docker.stdout.trim();
}

function baseCTE(testRunID) {
  const marker = sqlString(testRunID);
  const prefix = sqlString(`[E2E_TEST_${testRunID}]`);
  return `
WITH marker_patterns AS (
  SELECT ARRAY[
    '%' || ${marker} || '%',
    '%' || ${prefix} || '%'
  ]::text[] AS patterns
),
test_customers AS (
  SELECT id FROM customers, marker_patterns
  WHERE name ILIKE ANY(patterns)
     OR id ILIKE ANY(patterns)
     OR wecom_name ILIKE ANY(patterns)
     OR mobile_masked ILIKE ANY(patterns)
     OR source_channel ILIKE ANY(patterns)
     OR source_store_id ILIKE ANY(patterns)
     OR source_store_name ILIKE ANY(patterns)
     OR store_id ILIKE ANY(patterns)
     OR store_name ILIKE ANY(patterns)
     OR owner_staff_id ILIKE ANY(patterns)
     OR owner_staff_name ILIKE ANY(patterns)
     OR owner_guide_id ILIKE ANY(patterns)
     OR owner_guide_name ILIKE ANY(patterns)
),
test_tags AS (
  SELECT id FROM tags, marker_patterns
  WHERE id ILIKE ANY(patterns)
     OR name ILIKE ANY(patterns)
     OR source ILIKE ANY(patterns)
     OR category ILIKE ANY(patterns)
     OR wecom_tag_id ILIKE ANY(patterns)
),
test_tag_groups AS (
  SELECT id FROM tag_groups, marker_patterns
  WHERE id ILIKE ANY(patterns)
     OR name ILIKE ANY(patterns)
     OR scope ILIKE ANY(patterns)
),
test_sop_tasks AS (
  SELECT id FROM sop_tasks, marker_patterns
  WHERE id ILIKE ANY(patterns)
     OR title ILIKE ANY(patterns)
     OR description ILIKE ANY(patterns)
     OR source ILIKE ANY(patterns)
     OR customer_id IN (SELECT id FROM test_customers)
     OR customer_name ILIKE ANY(patterns)
     OR store_id ILIKE ANY(patterns)
     OR store_name ILIKE ANY(patterns)
     OR assigned_to_user_id ILIKE ANY(patterns)
     OR assigned_to_name ILIKE ANY(patterns)
),
test_stores AS (
  SELECT id FROM stores, marker_patterns
  WHERE id ILIKE ANY(patterns)
     OR name ILIKE ANY(patterns)
     OR internal_code ILIKE ANY(patterns)
     OR external_code ILIKE ANY(patterns)
),
test_guides AS (
  SELECT id FROM guides, marker_patterns
  WHERE id ILIKE ANY(patterns)
     OR store_id IN (SELECT id FROM test_stores)
     OR name ILIKE ANY(patterns)
     OR code ILIKE ANY(patterns)
     OR handling ILIKE ANY(patterns)
),
test_scrm_stores AS (
  SELECT id FROM scrm_stores, marker_patterns
  WHERE id ILIKE ANY(patterns)
     OR store_code ILIKE ANY(patterns)
     OR store_name ILIKE ANY(patterns)
     OR manager_userid ILIKE ANY(patterns)
),
test_scrm_bindings AS (
  SELECT id FROM scrm_wecom_contact_way_bindings, marker_patterns
  WHERE id ILIKE ANY(patterns)
     OR store_id IN (SELECT id FROM test_scrm_stores)
     OR manager_userid ILIKE ANY(patterns)
     OR config_id ILIKE ANY(patterns)
     OR state_prefix ILIKE ANY(patterns)
     OR remark ILIKE ANY(patterns)
     OR last_error ILIKE ANY(patterns)
),
test_wecom_configs AS (
  SELECT corp_id FROM wecom_configs, marker_patterns
  WHERE id ILIKE ANY(patterns)
     OR corp_id ILIKE ANY(patterns)
     OR agent_id ILIKE ANY(patterns)
),
test_legacy_wecom_configs AS (
  SELECT corp_id FROM wecom_corp_config, marker_patterns
  WHERE id ILIKE ANY(patterns)
     OR corp_id ILIKE ANY(patterns)
     OR agent_id ILIKE ANY(patterns)
     OR callback_url ILIKE ANY(patterns)
     OR test_department_id ILIKE ANY(patterns)
)
`;
}

function reportSelect(testRunID) {
  return `${baseCTE(testRunID)}
SELECT jsonb_pretty(jsonb_build_object(
  'operation_exceptions', (SELECT count(*) FROM operation_exceptions, marker_patterns WHERE id ILIKE ANY(patterns) OR exception_type ILIKE ANY(patterns) OR title ILIKE ANY(patterns) OR description ILIKE ANY(patterns) OR suggestion ILIKE ANY(patterns) OR customer_id IN (SELECT id FROM test_customers) OR task_id IN (SELECT id FROM test_sop_tasks) OR store_id ILIKE ANY(patterns)),
  'customer_operation_timeline', (SELECT count(*) FROM customer_operation_timeline, marker_patterns WHERE id ILIKE ANY(patterns) OR customer_id IN (SELECT id FROM test_customers) OR title ILIKE ANY(patterns) OR content ILIKE ANY(patterns) OR related_id ILIKE ANY(patterns) OR operator_id ILIKE ANY(patterns) OR operator_name ILIKE ANY(patterns)),
  'sop_task_logs', (SELECT count(*) FROM sop_task_logs, marker_patterns WHERE id ILIKE ANY(patterns) OR task_id IN (SELECT id FROM test_sop_tasks) OR remark ILIKE ANY(patterns) OR operator_id ILIKE ANY(patterns) OR operator_name ILIKE ANY(patterns)),
  'sop_tasks', (SELECT count(*) FROM test_sop_tasks),
  'follow_up_records', (SELECT count(*) FROM follow_up_records, marker_patterns WHERE id ILIKE ANY(patterns) OR customer_id IN (SELECT id FROM test_customers) OR content ILIKE ANY(patterns) OR result ILIKE ANY(patterns) OR created_by ILIKE ANY(patterns)),
  'customer_lifecycle_logs', (SELECT count(*) FROM customer_lifecycle_logs, marker_patterns WHERE id ILIKE ANY(patterns) OR customer_id IN (SELECT id FROM test_customers) OR reason ILIKE ANY(patterns) OR operator_id ILIKE ANY(patterns) OR operator_name ILIKE ANY(patterns)),
  'customer_tags', (SELECT count(*) FROM customer_tags WHERE customer_id IN (SELECT id FROM test_customers) OR tag_id IN (SELECT id FROM test_tags) OR source ILIKE ANY(ARRAY(SELECT unnest(patterns) FROM marker_patterns)) OR operator_id ILIKE ANY(ARRAY(SELECT unnest(patterns) FROM marker_patterns))),
  'customer_events', (SELECT count(*) FROM customer_events, marker_patterns WHERE id ILIKE ANY(patterns) OR customer_id IN (SELECT id FROM test_customers) OR source ILIKE ANY(patterns) OR object_id ILIKE ANY(patterns) OR payload::text ILIKE ANY(patterns)),
  'customer_staff_relations', (SELECT count(*) FROM customer_staff_relations, marker_patterns WHERE id ILIKE ANY(patterns) OR customer_id IN (SELECT id FROM test_customers) OR staff_id ILIKE ANY(patterns) OR staff_name ILIKE ANY(patterns) OR via_code_id ILIKE ANY(patterns) OR store_id ILIKE ANY(patterns)),
  'customer_attributions', (SELECT count(*) FROM customer_attributions, marker_patterns WHERE customer_id IN (SELECT id FROM test_customers) OR source_store_id ILIKE ANY(patterns) OR first_staff_id ILIKE ANY(patterns) OR source_code_id ILIKE ANY(patterns) OR entry_mode ILIKE ANY(patterns)),
  'customer_identities', (SELECT count(*) FROM customer_identities, marker_patterns WHERE customer_id IN (SELECT id FROM test_customers) OR identity_value ILIKE ANY(patterns)),
  'customers', (SELECT count(*) FROM test_customers),
  'tags', (SELECT count(*) FROM test_tags),
  'tag_groups', (SELECT count(*) FROM test_tag_groups),
  'guide_events', (SELECT count(*) FROM guide_events, marker_patterns WHERE id ILIKE ANY(patterns) OR guide_id IN (SELECT id FROM test_guides) OR detail ILIKE ANY(patterns) OR people ILIKE ANY(patterns) OR operator ILIKE ANY(patterns)),
  'guides', (SELECT count(*) FROM test_guides),
  'stores', (SELECT count(*) FROM test_stores),
  'scrm_wecom_contact_way_retry_tasks', (SELECT count(*) FROM scrm_wecom_contact_way_retry_tasks, marker_patterns WHERE id ILIKE ANY(patterns) OR binding_id IN (SELECT id FROM test_scrm_bindings) OR config_id ILIKE ANY(patterns) OR target_guide_userid ILIKE ANY(patterns) OR target_state ILIKE ANY(patterns) OR failed_reason ILIKE ANY(patterns)),
  'scrm_customer_assignments', (SELECT count(*) FROM scrm_customer_assignments, marker_patterns WHERE id ILIKE ANY(patterns) OR store_id IN (SELECT id FROM test_scrm_stores) OR binding_id IN (SELECT id FROM test_scrm_bindings) OR config_id ILIKE ANY(patterns) OR external_userid ILIKE ANY(patterns) OR state ILIKE ANY(patterns) OR raw_event::text ILIKE ANY(patterns)),
  'scrm_wecom_callback_events', (SELECT count(*) FROM scrm_wecom_callback_events, marker_patterns WHERE id ILIKE ANY(patterns) OR event_key ILIKE ANY(patterns) OR external_userid ILIKE ANY(patterns) OR user_id ILIKE ANY(patterns) OR state ILIKE ANY(patterns) OR raw_event::text ILIKE ANY(patterns)),
  'scrm_wecom_contact_way_bindings', (SELECT count(*) FROM test_scrm_bindings),
  'scrm_store_guides', (SELECT count(*) FROM scrm_store_guides, marker_patterns WHERE id ILIKE ANY(patterns) OR store_id IN (SELECT id FROM test_scrm_stores) OR guide_userid ILIKE ANY(patterns) OR guide_name ILIKE ANY(patterns)),
  'scrm_stores', (SELECT count(*) FROM test_scrm_stores),
  'wecom_permission_checks', (SELECT count(*) FROM wecom_permission_checks, marker_patterns WHERE id ILIKE ANY(patterns) OR check_type ILIKE ANY(patterns) OR errmsg ILIKE ANY(patterns) OR local_code ILIKE ANY(patterns) OR suggestion ILIKE ANY(patterns)),
  'wecom_tokens', (SELECT count(*) FROM wecom_tokens, marker_patterns WHERE corp_id IN (SELECT corp_id FROM test_wecom_configs) OR corp_id ILIKE ANY(patterns) OR token_type ILIKE ANY(patterns) OR last_error_code ILIKE ANY(patterns) OR last_error_message ILIKE ANY(patterns)),
  'wecom_configs', (SELECT count(*) FROM test_wecom_configs),
  'wecom_customer_events', (SELECT count(*) FROM wecom_customer_events, marker_patterns WHERE corp_id IN (SELECT corp_id FROM test_legacy_wecom_configs) OR corp_id ILIKE ANY(patterns) OR external_userid ILIKE ANY(patterns) OR follow_userid ILIKE ANY(patterns) OR state ILIKE ANY(patterns) OR contact_way_id ILIKE ANY(patterns) OR raw_payload::text ILIKE ANY(patterns)),
  'wecom_contact_ways', (SELECT count(*) FROM wecom_contact_ways, marker_patterns WHERE corp_id IN (SELECT corp_id FROM test_legacy_wecom_configs) OR corp_id ILIKE ANY(patterns) OR config_id ILIKE ANY(patterns) OR name ILIKE ANY(patterns) OR state ILIKE ANY(patterns) OR created_by ILIKE ANY(patterns)),
  'wecom_users', (SELECT count(*) FROM wecom_users, marker_patterns WHERE corp_id IN (SELECT corp_id FROM test_legacy_wecom_configs) OR corp_id ILIKE ANY(patterns) OR userid ILIKE ANY(patterns) OR name ILIKE ANY(patterns) OR department_id ILIKE ANY(patterns) OR department_name ILIKE ANY(patterns)),
  'wecom_access_tokens', (SELECT count(*) FROM wecom_access_tokens, marker_patterns WHERE corp_id IN (SELECT corp_id FROM test_legacy_wecom_configs) OR corp_id ILIKE ANY(patterns)),
  'wecom_corp_config', (SELECT count(*) FROM test_legacy_wecom_configs)
))::text;
`;
}

function applySQL(testRunID) {
  return `${baseCTE(testRunID)}
,del_operation_exceptions AS (
  DELETE FROM operation_exceptions USING marker_patterns
  WHERE id ILIKE ANY(patterns) OR exception_type ILIKE ANY(patterns) OR title ILIKE ANY(patterns) OR description ILIKE ANY(patterns) OR suggestion ILIKE ANY(patterns) OR customer_id IN (SELECT id FROM test_customers) OR task_id IN (SELECT id FROM test_sop_tasks) OR store_id ILIKE ANY(patterns)
  RETURNING 1
),
del_customer_operation_timeline AS (
  DELETE FROM customer_operation_timeline USING marker_patterns
  WHERE id ILIKE ANY(patterns) OR customer_id IN (SELECT id FROM test_customers) OR title ILIKE ANY(patterns) OR content ILIKE ANY(patterns) OR related_id ILIKE ANY(patterns) OR operator_id ILIKE ANY(patterns) OR operator_name ILIKE ANY(patterns)
  RETURNING 1
),
del_sop_task_logs AS (
  DELETE FROM sop_task_logs USING marker_patterns
  WHERE id ILIKE ANY(patterns) OR task_id IN (SELECT id FROM test_sop_tasks) OR remark ILIKE ANY(patterns) OR operator_id ILIKE ANY(patterns) OR operator_name ILIKE ANY(patterns)
  RETURNING 1
),
del_sop_tasks AS (
  DELETE FROM sop_tasks WHERE id IN (SELECT id FROM test_sop_tasks)
  RETURNING 1
),
del_follow_up_records AS (
  DELETE FROM follow_up_records USING marker_patterns
  WHERE id ILIKE ANY(patterns) OR customer_id IN (SELECT id FROM test_customers) OR content ILIKE ANY(patterns) OR result ILIKE ANY(patterns) OR created_by ILIKE ANY(patterns)
  RETURNING 1
),
del_customer_lifecycle_logs AS (
  DELETE FROM customer_lifecycle_logs USING marker_patterns
  WHERE id ILIKE ANY(patterns) OR customer_id IN (SELECT id FROM test_customers) OR reason ILIKE ANY(patterns) OR operator_id ILIKE ANY(patterns) OR operator_name ILIKE ANY(patterns)
  RETURNING 1
),
del_customer_tags AS (
  DELETE FROM customer_tags
  WHERE customer_id IN (SELECT id FROM test_customers)
     OR tag_id IN (SELECT id FROM test_tags)
     OR source ILIKE ANY(ARRAY(SELECT unnest(patterns) FROM marker_patterns))
     OR operator_id ILIKE ANY(ARRAY(SELECT unnest(patterns) FROM marker_patterns))
  RETURNING 1
),
del_customer_events AS (
  DELETE FROM customer_events USING marker_patterns
  WHERE id ILIKE ANY(patterns) OR customer_id IN (SELECT id FROM test_customers) OR source ILIKE ANY(patterns) OR object_id ILIKE ANY(patterns) OR payload::text ILIKE ANY(patterns)
  RETURNING 1
),
del_customer_staff_relations AS (
  DELETE FROM customer_staff_relations USING marker_patterns
  WHERE id ILIKE ANY(patterns) OR customer_id IN (SELECT id FROM test_customers) OR staff_id ILIKE ANY(patterns) OR staff_name ILIKE ANY(patterns) OR via_code_id ILIKE ANY(patterns) OR store_id ILIKE ANY(patterns)
  RETURNING 1
),
del_customer_attributions AS (
  DELETE FROM customer_attributions USING marker_patterns
  WHERE customer_id IN (SELECT id FROM test_customers) OR source_store_id ILIKE ANY(patterns) OR first_staff_id ILIKE ANY(patterns) OR source_code_id ILIKE ANY(patterns) OR entry_mode ILIKE ANY(patterns)
  RETURNING 1
),
del_customer_identities AS (
  DELETE FROM customer_identities USING marker_patterns
  WHERE customer_id IN (SELECT id FROM test_customers) OR identity_value ILIKE ANY(patterns)
  RETURNING 1
),
del_customers AS (
  DELETE FROM customers WHERE id IN (SELECT id FROM test_customers)
  RETURNING 1
),
del_tags AS (
  DELETE FROM tags WHERE id IN (SELECT id FROM test_tags)
  RETURNING 1
),
del_tag_groups AS (
  DELETE FROM tag_groups WHERE id IN (SELECT id FROM test_tag_groups)
  RETURNING 1
),
del_guide_events AS (
  DELETE FROM guide_events USING marker_patterns
  WHERE id ILIKE ANY(patterns) OR guide_id IN (SELECT id FROM test_guides) OR detail ILIKE ANY(patterns) OR people ILIKE ANY(patterns) OR operator ILIKE ANY(patterns)
  RETURNING 1
),
del_guides AS (
  DELETE FROM guides WHERE id IN (SELECT id FROM test_guides)
  RETURNING 1
),
del_stores AS (
  DELETE FROM stores WHERE id IN (SELECT id FROM test_stores)
  RETURNING 1
),
del_scrm_retries AS (
  DELETE FROM scrm_wecom_contact_way_retry_tasks USING marker_patterns
  WHERE id ILIKE ANY(patterns) OR binding_id IN (SELECT id FROM test_scrm_bindings) OR config_id ILIKE ANY(patterns) OR target_guide_userid ILIKE ANY(patterns) OR target_state ILIKE ANY(patterns) OR failed_reason ILIKE ANY(patterns)
  RETURNING 1
),
del_scrm_assignments AS (
  DELETE FROM scrm_customer_assignments USING marker_patterns
  WHERE id ILIKE ANY(patterns) OR store_id IN (SELECT id FROM test_scrm_stores) OR binding_id IN (SELECT id FROM test_scrm_bindings) OR config_id ILIKE ANY(patterns) OR external_userid ILIKE ANY(patterns) OR state ILIKE ANY(patterns) OR raw_event::text ILIKE ANY(patterns)
  RETURNING 1
),
del_scrm_callbacks AS (
  DELETE FROM scrm_wecom_callback_events USING marker_patterns
  WHERE id ILIKE ANY(patterns) OR event_key ILIKE ANY(patterns) OR external_userid ILIKE ANY(patterns) OR user_id ILIKE ANY(patterns) OR state ILIKE ANY(patterns) OR raw_event::text ILIKE ANY(patterns)
  RETURNING 1
),
del_scrm_bindings AS (
  DELETE FROM scrm_wecom_contact_way_bindings WHERE id IN (SELECT id FROM test_scrm_bindings)
  RETURNING 1
),
del_scrm_guides AS (
  DELETE FROM scrm_store_guides USING marker_patterns
  WHERE id ILIKE ANY(patterns) OR store_id IN (SELECT id FROM test_scrm_stores) OR guide_userid ILIKE ANY(patterns) OR guide_name ILIKE ANY(patterns)
  RETURNING 1
),
del_scrm_stores AS (
  DELETE FROM scrm_stores WHERE id IN (SELECT id FROM test_scrm_stores)
  RETURNING 1
),
del_wecom_permission_checks AS (
  DELETE FROM wecom_permission_checks USING marker_patterns
  WHERE id ILIKE ANY(patterns) OR check_type ILIKE ANY(patterns) OR errmsg ILIKE ANY(patterns) OR local_code ILIKE ANY(patterns) OR suggestion ILIKE ANY(patterns)
  RETURNING 1
),
del_wecom_tokens AS (
  DELETE FROM wecom_tokens USING marker_patterns
  WHERE corp_id IN (SELECT corp_id FROM test_wecom_configs) OR corp_id ILIKE ANY(patterns) OR token_type ILIKE ANY(patterns) OR last_error_code ILIKE ANY(patterns) OR last_error_message ILIKE ANY(patterns)
  RETURNING 1
),
del_wecom_configs AS (
  DELETE FROM wecom_configs WHERE corp_id IN (SELECT corp_id FROM test_wecom_configs)
  RETURNING 1
),
del_legacy_wecom_customer_events AS (
  DELETE FROM wecom_customer_events USING marker_patterns
  WHERE corp_id IN (SELECT corp_id FROM test_legacy_wecom_configs) OR corp_id ILIKE ANY(patterns) OR external_userid ILIKE ANY(patterns) OR follow_userid ILIKE ANY(patterns) OR state ILIKE ANY(patterns) OR contact_way_id ILIKE ANY(patterns) OR raw_payload::text ILIKE ANY(patterns)
  RETURNING 1
),
del_legacy_wecom_contact_ways AS (
  DELETE FROM wecom_contact_ways USING marker_patterns
  WHERE corp_id IN (SELECT corp_id FROM test_legacy_wecom_configs) OR corp_id ILIKE ANY(patterns) OR config_id ILIKE ANY(patterns) OR name ILIKE ANY(patterns) OR state ILIKE ANY(patterns) OR created_by ILIKE ANY(patterns)
  RETURNING 1
),
del_legacy_wecom_users AS (
  DELETE FROM wecom_users USING marker_patterns
  WHERE corp_id IN (SELECT corp_id FROM test_legacy_wecom_configs) OR corp_id ILIKE ANY(patterns) OR userid ILIKE ANY(patterns) OR name ILIKE ANY(patterns) OR department_id ILIKE ANY(patterns) OR department_name ILIKE ANY(patterns)
  RETURNING 1
),
del_legacy_wecom_access_tokens AS (
  DELETE FROM wecom_access_tokens USING marker_patterns
  WHERE corp_id IN (SELECT corp_id FROM test_legacy_wecom_configs) OR corp_id ILIKE ANY(patterns)
  RETURNING 1
),
del_legacy_wecom_corp_config AS (
  DELETE FROM wecom_corp_config WHERE corp_id IN (SELECT corp_id FROM test_legacy_wecom_configs)
  RETURNING 1
)
SELECT jsonb_pretty(jsonb_build_object(
  'operation_exceptions', (SELECT count(*) FROM del_operation_exceptions),
  'customer_operation_timeline', (SELECT count(*) FROM del_customer_operation_timeline),
  'sop_task_logs', (SELECT count(*) FROM del_sop_task_logs),
  'sop_tasks', (SELECT count(*) FROM del_sop_tasks),
  'follow_up_records', (SELECT count(*) FROM del_follow_up_records),
  'customer_lifecycle_logs', (SELECT count(*) FROM del_customer_lifecycle_logs),
  'customer_tags', (SELECT count(*) FROM del_customer_tags),
  'customer_events', (SELECT count(*) FROM del_customer_events),
  'customer_staff_relations', (SELECT count(*) FROM del_customer_staff_relations),
  'customer_attributions', (SELECT count(*) FROM del_customer_attributions),
  'customer_identities', (SELECT count(*) FROM del_customer_identities),
  'customers', (SELECT count(*) FROM del_customers),
  'tags', (SELECT count(*) FROM del_tags),
  'tag_groups', (SELECT count(*) FROM del_tag_groups),
  'guide_events', (SELECT count(*) FROM del_guide_events),
  'guides', (SELECT count(*) FROM del_guides),
  'stores', (SELECT count(*) FROM del_stores),
  'scrm_wecom_contact_way_retry_tasks', (SELECT count(*) FROM del_scrm_retries),
  'scrm_customer_assignments', (SELECT count(*) FROM del_scrm_assignments),
  'scrm_wecom_callback_events', (SELECT count(*) FROM del_scrm_callbacks),
  'scrm_wecom_contact_way_bindings', (SELECT count(*) FROM del_scrm_bindings),
  'scrm_store_guides', (SELECT count(*) FROM del_scrm_guides),
  'scrm_stores', (SELECT count(*) FROM del_scrm_stores),
  'wecom_permission_checks', (SELECT count(*) FROM del_wecom_permission_checks),
  'wecom_tokens', (SELECT count(*) FROM del_wecom_tokens),
  'wecom_configs', (SELECT count(*) FROM del_wecom_configs),
  'wecom_customer_events', (SELECT count(*) FROM del_legacy_wecom_customer_events),
  'wecom_contact_ways', (SELECT count(*) FROM del_legacy_wecom_contact_ways),
  'wecom_users', (SELECT count(*) FROM del_legacy_wecom_users),
  'wecom_access_tokens', (SELECT count(*) FROM del_legacy_wecom_access_tokens),
  'wecom_corp_config', (SELECT count(*) FROM del_legacy_wecom_corp_config)
))::text;
`;
}

function parseJSONOutput(text) {
  const start = text.indexOf("{");
  const end = text.lastIndexOf("}");
  if (start === -1 || end === -1 || end < start) {
    throw new Error(`psql did not return JSON: ${text}`);
  }
  return JSON.parse(text.slice(start, end + 1));
}

function totalCount(report) {
  return Object.values(report).reduce((sum, value) => sum + Number(value || 0), 0);
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  if (args.help) {
    usage();
    return;
  }
  const databaseURL = assertSafeInput(args);
  const dryRun = parseJSONOutput(runPSQL(databaseURL, reportSelect(args.testRunID)));
  const result = {
    test_run_id: args.testRunID,
    mode: args.mode,
    database: new URL(databaseURL).pathname.replace(/^\//, ""),
    touched_15432_volume: false,
    dry_run: dryRun,
  };
  if (args.mode === "apply") {
    result.deleted = parseJSONOutput(runPSQL(databaseURL, applySQL(args.testRunID)));
    result.remaining = parseJSONOutput(runPSQL(databaseURL, reportSelect(args.testRunID)));
    result.cleanup_passed = totalCount(result.remaining) === 0;
    if (!result.cleanup_passed) {
      result.warning = "cleanup left matching rows. Inspect the remaining counts before running more tests.";
      console.log(JSON.stringify(result, null, 2));
      process.exitCode = 1;
      return;
    }
  }
  console.log(JSON.stringify(result, null, 2));
}

main().catch((err) => {
  console.error(`e2e cleanup failed: ${err.message}`);
  process.exit(1);
});
