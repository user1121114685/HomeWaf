import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { BuildInputs } from "./dev-inputs.mjs";

function fixture(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "homewaf-inputs-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const put = (name, value) => {
    fs.mkdirSync(path.dirname(path.join(root, name)), { recursive: true });
    fs.writeFileSync(path.join(root, name), value);
  };
  put("engine/main.go", "package engine\n");
  return { root, put, inputs: new BuildInputs(root) };
}

test("touches, same-byte saves, reads and irrelevant events never rebuild", (t) => {
  const { root, put, inputs } = fixture(t);
  const source = path.join(root, "engine/main.go");
  fs.readFileSync(source);
  fs.utimesSync(source, new Date(), new Date(Date.now() + 10000));
  assert.deepEqual(inputs.scan(), []);
  put("engine/main.go", "package engine\n");
  put("engine/main_test.go", "package engine\n");
  put("engine/readme.md", "notes");
  put(".cache/dev/runtime/data/run.go", "ignored");
  put("web/src/app.go", "ignored");
  fs.mkdirSync(path.join(root, "empty"));
  assert.deepEqual(inputs.scan(), []);
  assert.deepEqual(inputs.scan(), []);
});

test("content changes and directory add, rename and delete reconcile actual inputs", (t) => {
  const { root, put, inputs } = fixture(t);
  put("engine/main.go", "package changed\n");
  assert.deepEqual(inputs.scan(), ["engine/main.go"]);
  put("new/package.go", "package added\n");
  assert.deepEqual(inputs.scan(), ["new/package.go"]);
  fs.renameSync(path.join(root, "new"), path.join(root, "moved"));
  assert.deepEqual(inputs.scan(), ["moved/package.go", "new/package.go"]);
  fs.unlinkSync(path.join(root, "moved/package.go"));
  assert.deepEqual(inputs.scan(), ["moved/package.go"]);
  assert.deepEqual(inputs.scan(), []);
});

test("module and embedded assets trigger only on content changes", (t) => {
  const { put, inputs } = fixture(t);
  const names = ["go.mod", "go.sum", "public/dist/.gitkeep", "public/dist/assets/app.js", "public/web_frontend.txt", "cmd/homewaf/exedata/capjs/index.js", "wafmangeweb/emergency_page.html"];
  for (const name of names) put(name, "v1");
  assert.deepEqual(inputs.scan(), names.sort());
  for (const name of names) put(name, "v1");
  assert.deepEqual(inputs.scan(), []);
  put("public/dist/assets/app.js", "v2");
  assert.deepEqual(inputs.scan(), ["public/dist/assets/app.js"]);
});
