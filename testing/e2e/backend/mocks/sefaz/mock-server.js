const http = require("node:http");
const fs = require("node:fs/promises");
const path = require("node:path");

const port = Number(process.env.PORT || 8091);
const baseDir = path.join(__dirname, "mocks");

const routes = {
  "/nfce-consulta-detalhada.html": "nfce-consulta-detalhada.html",
  "/captcha.html": "captcha.html",
};

const server = http.createServer(async (req, res) => {
  const requestUrl = new URL(req.url || "/", `http://localhost:${port}`);
  const url = requestUrl.pathname;

  if (url === "/health") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ status: "ok" }));
    return;
  }

  if (url === "/slow.html") {
    await new Promise((resolve) => setTimeout(resolve, 7000));
    const filePath = path.join(baseDir, "nfce-consulta-detalhada.html");
    const html = await fs.readFile(filePath, "utf8");
    res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
    res.end(html);
    return;
  }

  const fileName = routes[url];
  if (!fileName) {
    res.writeHead(404, { "Content-Type": "text/plain; charset=utf-8" });
    res.end("Not found");
    return;
  }

  const filePath = path.join(baseDir, fileName);
  const html = await fs.readFile(filePath, "utf8");
  res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
  res.end(html);
});

server.listen(port, "0.0.0.0", () => {
  console.log(`SEFAZ mock server listening on ${port}`);
});
