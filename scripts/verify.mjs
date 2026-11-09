import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
process.chdir(fileURLToPath(new URL("..", import.meta.url)));
function run(args) {
  const result = spawnSync("go", args, { stdio: "inherit" });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status || 1);
}
// Compile every production and test package; execute only focused regressions.
run(["test", "./...", "-run", "^$"]);
run([
  "test",
  "./internal/licensing",
  "./internal/thirdparty/capserver",
  "./internal/sqlite3",
  "./internal/mysqldriver",
  "./common/gwebsocket",
  "./wafinit",
  "./wafenginecore/wafcaptcha",
  "-count=1",
]);
run([
  "test",
  "./utils",
  "./model",
  "./wafenginecore",
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
run(["test", "./wafupdate", "./wafupgradenotice", "./wafai", "-count=1"]);
run(["test", "./wafnet", "./supervisor", "-count=1"]);
run(["test", "./wafenginecore", "./cmd/homewaf", "-run", "Test(ProxyReadiness|WorkerReadiness|Graceful)", "-count=1"]);
