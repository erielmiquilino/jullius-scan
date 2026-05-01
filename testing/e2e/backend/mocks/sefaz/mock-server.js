const http = require("node:http");
const fs = require("node:fs/promises");
const path = require("node:path");

const port = Number(process.env.PORT || 8091);
// In Docker, HTML fixtures are mounted at /app/mocks (see docker-compose.e2e.yml).
// Locally, they live next to this file in the same directory.
const baseDir = process.env.MOCK_HTML_DIR || path.join(__dirname);

const routes = {
  "/nfce-consulta-detalhada.html": "nfce-consulta-detalhada.html",
  "/captcha.html": "captcha.html",
  "/tax.NET/Sat.NFe.Web/Consultas/Nfe_DetalheCert.aspx": "nfce-detalhe-cert.html",
  "/nfce-captcha-detail-summary.html": "nfce-captcha-detail-summary.html",
};

// Parses the Cookie header into a key/value map.
function parseCookies(cookieHeader) {
  if (!cookieHeader) return {};
  return Object.fromEntries(
    cookieHeader.split(";").map((c) => {
      const [k, ...v] = c.trim().split("=");
      return [k.trim(), v.join("=").trim()];
    }),
  );
}

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

  // Captcha challenge route used by E2E tests.
  // Returns the challenge page unless the request carries the magic cookie
  // `e2e_captcha_solved=1`, in which case it serves the real NFC-e HTML.
  if (url === "/nfce/captcha-challenge") {
    const cookies = parseCookies(req.headers["cookie"]);
    if (cookies["e2e_captcha_solved"] === "1") {
      const filePath = path.join(baseDir, "nfce-consulta-detalhada.html");
      const html = await fs.readFile(filePath, "utf8");
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(html);
    } else {
      // Set a session cookie so subsequent requests from the same browser session
      // can be upgraded by adding e2e_captcha_solved=1.
      res.writeHead(200, {
        "Content-Type": "text/html; charset=utf-8",
        "Set-Cookie": "e2e_session=pending; Path=/; HttpOnly",
      });
      const filePath = path.join(baseDir, "captcha-challenge.html");
      const html = await fs.readFile(filePath, "utf8");
      res.end(html);
    }
    return;
  }

  // Detail-page captcha route: serves captcha on first visit, detail page after.
  // The summary page at /nfce-captcha-detail-summary.html links here.
  // Used for E2E testing of captcha pauses during the summary→detail transition.
  if (url === "/nfce/detail-challenge") {
    const cookies = parseCookies(req.headers["cookie"]);
    if (cookies["e2e_captcha_solved"] === "1") {
      const filePath = path.join(baseDir, "nfce-detalhe-cert.html");
      const html = await fs.readFile(filePath, "utf8");
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(html);
    } else {
      res.writeHead(200, {
        "Content-Type": "text/html; charset=utf-8",
        "Set-Cookie": "e2e_session=pending; Path=/; HttpOnly",
      });
      const filePath = path.join(baseDir, "captcha-challenge.html");
      const html = await fs.readFile(filePath, "utf8");
      res.end(html);
    }
    return;
  }

  if (url === "/tax.NET/Sat.NFe.Web/Consultas/Nfe_DetalheCert.aspx" && requestUrl.searchParams.get("rq") === "DETAIL_GATE_TOKEN") {
    const cookies = parseCookies(req.headers["cookie"]);
    if (cookies["e2e_captcha_solved"] === "1") {
      const filePath = path.join(baseDir, "nfce-detalhe-cert.html");
      const html = await fs.readFile(filePath, "utf8");
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(html);
    } else {
      res.writeHead(200, {
        "Content-Type": "text/html; charset=utf-8",
        "Set-Cookie": "e2e_session=pending; Path=/; HttpOnly",
      });
      const filePath = path.join(baseDir, "captcha-challenge.html");
      const html = await fs.readFile(filePath, "utf8");
      res.end(html);
    }
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
