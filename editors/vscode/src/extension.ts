import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from "vscode-languageclient/node";

let client: LanguageClient | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  const config = () => vscode.workspace.getConfiguration("buddy");

  if (config().get<boolean>("associateStar")) {
    context.subscriptions.push(
      vscode.workspace.onDidOpenTextDocument(async (doc) => {
        if (doc.uri.scheme === "file" && doc.fileName.endsWith(".star") && doc.languageId !== "buddy") {
          await vscode.languages.setTextDocumentLanguage(doc, "buddy");
        }
      })
    );
    for (const doc of vscode.workspace.textDocuments) {
      if (doc.uri.scheme === "file" && doc.fileName.endsWith(".star") && doc.languageId !== "buddy") {
        void vscode.languages.setTextDocumentLanguage(doc, "buddy");
      }
    }
  }

  const buddyPath = config().get<string>("path") || "buddy";
  const lspOverride = (config().get<string>("lsp.path") || "").trim();
  const cellMode = config().get<boolean>("cellMode") === true;

  let serverOptions: ServerOptions;
  if (lspOverride) {
    const parts = lspOverride.split(/\s+/).filter(Boolean);
    serverOptions = {
      command: parts[0],
      args: parts.slice(1),
      transport: TransportKind.stdio,
    };
  } else {
    serverOptions = {
      command: buddyPath,
      args: ["lsp"],
      transport: TransportKind.stdio,
    };
  }

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: "file", language: "buddy" }],
    synchronize: {
      configurationSection: "buddy",
    },
    initializationOptions: {
      cellMode,
    },
  };

  client = new LanguageClient("buddy", "Buddy Starlark", serverOptions, clientOptions);
  context.subscriptions.push(client);
  await client.start();

  context.subscriptions.push(
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("buddy")) {
        void vscode.window.showInformationMessage(
          "Buddy settings changed. Reload the window to apply language server options."
        );
      }
    })
  );
}

export async function deactivate(): Promise<void> {
  if (client) {
    await client.stop();
  }
}
