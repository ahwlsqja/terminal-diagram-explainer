import { createReadStream } from "node:fs";
import { stat } from "node:fs/promises";
import { createServer } from "node:http";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const port = Number.parseInt(process.env.PORT ?? "4173", 10);
const routes = new Map([
  ["/", path.join(root, "test/host-harness.html")],
  ["/host-harness.html", path.join(root, "test/host-harness.html")],
  ["/dist/widget.html", path.join(root, "dist/widget.html")],
]);

createServer(async (request, response) => {
  const pathname = new URL(request.url ?? "/", `http://${request.headers.host}`).pathname;
  const file = routes.get(pathname);
  if (!file) {
    response.writeHead(404, { "content-type": "text/plain; charset=utf-8" });
    response.end("Not found\n");
    return;
  }
  try {
    const info = await stat(file);
    response.writeHead(200, {
      "content-type": "text/html; charset=utf-8",
      "content-length": info.size,
      "cache-control": "no-store",
    });
    createReadStream(file).pipe(response);
  } catch {
    response.writeHead(500, { "content-type": "text/plain; charset=utf-8" });
    response.end("Missing build artifact\n");
  }
}).listen(port, "127.0.0.1", () => {
  console.log(`Diagram test host: http://127.0.0.1:${port}`);
});
