import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
process.chdir(fileURLToPath(new URL("..", import.meta.url)));
process.env.CGO_ENABLED ||= "0";
const devInputs = spawnSync(process.execPath, ["--test", "scripts/dev-inputs.test.mjs"], { stdio: "inherit" });
const buildVersions = spawnSync(process.execPath, ["--test", "scripts/build-version.test.mjs"], { stdio: "inherit" });
if (buildVersions.error) throw buildVersions.error;
if (buildVersions.status !== 0) process.exit(buildVersions.status || 1);
if (devInputs.error) throw devInputs.error;
if (devInputs.status !== 0) process.exit(devInputs.status || 1);
const prepare = spawnSync(process.execPath, ["scripts/prepare-ip-databases.mjs", "--check"], { stdio: "inherit" });
if (prepare.error) throw prepare.error;
if (prepare.status !== 0) process.exit(prepare.status || 1);
function run(args) {
  if (args[0] === "test") args.splice(1, 0, "-trimpath");
  const result = spawnSync("go", args, { stdio: "inherit" });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status || 1);
}
run(["test", "./wafdb", "./wafdb/dialect", "./waflogexport", "-short", "-count=1"]);
run(["test", "./service/waf_service", "-run", "^TestSecretServicesFailWithoutInstanceKey$", "-count=1"]);
run(["test", "./service/waf_service", "./utils", "./api", "./wafnotify/channels", "-run", "^Test(LogServicesIncludePipelineAndPreserveLogIdentity|LogRequestOutcomeFiltersObserveAndResponseFailures|SystemLogFilteringAndFailures|OutboundDNSFailureDoesNotRecommendAllowlistOrExposePath|NotifyProviderCreateEditTestAndSecretStorage|RejectsFailedOrMalformedProviderResponses)$", "-count=1"]);

// Compile every production and test package; execute only focused regressions.
run(["test", "./...", "-run", "^$"]);
// Keep opt-in database/performance tests compilable without connecting to a database server.
run(["test", "-tags=crossdb,iptagperf", "./...", "-run", "^$"]);
// Exercise current schema initialization and upserts using temporary SQLite databases only.
run(["test", "-tags=crossdb", "./service/waf_service", "-run", "^TestCrossEngine$/^sqlite$", "-count=1"]);
run([
  "test",
  "-tags=crossdb",
  "./waftask",
  "-run",
  "^Test(InitializeArchiveSchema|CollectAnalysisStatsCrossEngine|CollectStatsIPTagCrossEngine)$/^sqlite$",
  "-count=1",
]);
run([
  "test",
  "./internal/licensing",
  "./internal/secretfield",
  "./wafsec",
  "./wafreg",
  "./wafcli",
  "./internal/thirdparty/capserver",
  "./internal/sqlitedialect",
  "./internal/mysqldriver",
  "./utils/filelease",
  "./common/gwebsocket",
  "./wafinit",
  "./wafenginecore/wafcaptcha",
  "-count=1",
]);
run([
  "test",
  "./utils",
  "./model",
  "./wafqueue",
  "-run",
  "Test.*(Rule|HTTP3|Http3|WebSocket|Websocket|MessageQueue)",
  "-count=1",
]);

run([
  "test",
  "./api",
  "-run",
  "^Test(SignedLicenseImportBoundToSession|HomeWafUpdateSourceResponses)$",
  "-count=1",
]);
run(["test", "./wafupdate", "./wafupgradenotice", "-count=1"]);
run(["test", "./wafnet", "./supervisor", "-count=1"]);
run(["test", "./waftask", "-run", "^TestEveryCleanupTableHasSeededPolicy$", "-count=1"]);
run(["test", "./waftask", "-run", "^TestSSLOrderScheduledRenewal", "-count=1"]);
run(["test", "./middleware", "-run", "^TestOpenApiLocalOnlySchemaPreservesKeys$", "-count=1"]);
run(["test", "./cmd/homewaf", "-run", "Test(WorkerReadiness|Graceful)", "-count=1"]);
run(["run", "./scripts/generate-dns-providers", "-check"]);
run(["test", "./internal/dnsproviders", "./utils/ssl", "./service/waf_service", "./api", "./router", "-run", "^Test(DNS|Hurricane|SSLCertificate|ZeroSSL)", "-count=1"]);
run(["test", "./wafenginecore", "-short", "-count=1"]);
run(["test", "./utils", "-run", "^TestOutboundRequestError", "-count=1"]);
// Full handler/router boundary regressions use isolated fixtures only.
run(["test", "./api", "./router", "./middleware", "./wafmangeweb", "./wafmcp", "./enums", "-count=1"]);
run(["test", "./service/waf_service", "-run", "^Test(CertificateLifecycle|SSLOrder|SensitiveConfig|SystemConfigRejects)", "-count=1"]);
run(["test", "./wafpipeline", "-short", "-count=1"]);
run(["test", "./service/waf_service", "./firewall", "-run", "^Test(NewServiceRequiresEnabledDefense|Defense|FlowDrop)", "-count=1"]);
run(["test", "./internal/acmelog", "./internal/acmelease", "-count=1"]);
run(["test", "./utils/ssl", "-run", "^TestSSLOrderLegoLogger", "-count=1"]);
