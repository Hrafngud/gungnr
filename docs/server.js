const http = require('http');
const fs = require('fs');
const path = require('path');

const rootDir = __dirname;

const normalizeBaseUrl = (value) => value.trim().replace(/\/+$/, '');

const loadEnvFile = (filePath) => {
  if (!fs.existsSync(filePath)) return {};
  const contents = fs.readFileSync(filePath, 'utf8');
  const env = {};

  contents.split(/\r?\n/).forEach((line) => {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) return;
    const match = trimmed.match(/^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$/);
    if (!match) return;
    let [, key, value] = match;
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    env[key] = value;
  });

  return env;
};

const envFromFile = loadEnvFile(path.join(rootDir, '.env'));
const env = { ...envFromFile, ...process.env };

const host = env.DOCS_HOST || '127.0.0.1';
const port = Number(env.DOCS_PORT) || 8081;
const appBaseUrl = env.APP_BASE_URL
  ? normalizeBaseUrl(env.APP_BASE_URL)
  : `http://localhost:${port}`;

const contentTypes = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
  '.json': 'application/json; charset=utf-8',
};

const injectBaseUrl = (html) => {
  if (html.includes('window.APP_BASE_URL')) return html;
  const injection = `<script>window.APP_BASE_URL=${JSON.stringify(appBaseUrl)};</script>`;
  if (html.includes('</head>')) {
    return html.replace('</head>', `${injection}\n  </head>`);
  }
  return `${injection}\n${html}`;
};

const resolveFilePath = (pathname) => {
  const safePath = decodeURIComponent(pathname).replace(/\\0/g, '');
  const normalized = safePath === '/' ? '/index.html' : safePath;
  const resolved = path.normalize(path.join(rootDir, normalized));
  if (!resolved.startsWith(rootDir)) return null;
  return resolved;
};

const server = http.createServer((req, res) => {
  if (!req.url) {
    res.writeHead(400);
    res.end('Bad Request');
    return;
  }

  if (req.method !== 'GET' && req.method !== 'HEAD') {
    res.writeHead(405, { Allow: 'GET, HEAD' });
    res.end('Method Not Allowed');
    return;
  }

  let pathname = '/';
  try {
    pathname = new URL(req.url, `http://${host}`).pathname;
  } catch (err) {
    res.writeHead(400);
    res.end('Bad Request');
    return;
  }

  const filePath = resolveFilePath(pathname);
  if (!filePath) {
    res.writeHead(404);
    res.end('Not Found');
    return;
  }

  fs.stat(filePath, (err, stat) => {
    if (err || !stat.isFile()) {
      res.writeHead(404);
      res.end('Not Found');
      return;
    }

    const ext = path.extname(filePath).toLowerCase();
    const contentType = contentTypes[ext] || 'application/octet-stream';

    if (ext === '.html') {
      fs.readFile(filePath, 'utf8', (readErr, data) => {
        if (readErr) {
          res.writeHead(500);
          res.end('Server Error');
          return;
        }
        const html = injectBaseUrl(data);
        res.writeHead(200, { 'Content-Type': contentType });
        if (req.method === 'HEAD') {
          res.end();
          return;
        }
        res.end(html);
      });
      return;
    }

    res.writeHead(200, { 'Content-Type': contentType });
    if (req.method === 'HEAD') {
      res.end();
      return;
    }
    fs.createReadStream(filePath).pipe(res);
  });
});

server.listen(port, host, () => {
  console.log(`Docs server running at http://localhost:${port}`);
});
