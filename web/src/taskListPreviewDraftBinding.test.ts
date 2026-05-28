import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { describe, it } from "node:test";
import ts from "typescript";

const sourcePath = join(dirname(fileURLToPath(import.meta.url)), "main.tsx");

describe("task list preview draft binding", () => {
  it("destructures onPreviewDraft before passing it to task rows", () => {
    const source = ts.createSourceFile(
      sourcePath,
      readFileSync(sourcePath, "utf8"),
      ts.ScriptTarget.Latest,
      true,
      ts.ScriptKind.TSX,
    );

    const taskList = source.statements.find(
      (statement): statement is ts.FunctionDeclaration =>
        ts.isFunctionDeclaration(statement) && statement.name?.text === "TaskList",
    );
    assert.ok(taskList, "TaskList function should exist");

    const props = taskList.parameters[0]?.name;
    assert.ok(props && ts.isObjectBindingPattern(props), "TaskList should destructure props");

    const names = props.elements.map((element) => element.name.getText(source));
    assert.ok(
      names.includes("onPreviewDraft"),
      "TaskList must destructure onPreviewDraft so runtime JSX does not reference an undefined variable",
    );
  });
});
