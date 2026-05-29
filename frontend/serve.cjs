const http = require('http');
const fs = require('fs');
const path = require('path');
const DIST = path.join(__dirname, 'dist');
const API = 'http://localhost:8080';
const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.png': 'image/png', '.jpg': 'image/jpeg', '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon', '.woff2': 'font/woff2',
};
function serveStatic(req, res) {
  let fp = path.join(DIST, req.url === '/' ? 'index.html' : req.url);
  const ext = path.extname(fp);
  fs.readFile(fp, (err, data) => {
    if (err) {
      fs.readFile(path.join(DIST, 'index.html'), (err2, data2) => {
        if (err2) { res.writeHead(500); res.end('Error'); return; }
        res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
        res.end(data2);
      });
      return;
    }
    res.writeHead(200, { 'Content-Type': MIME[ext] || 'application/octet-stream' });
    res.end(data);
  });
}
http.createServer((req, res) => {
  if (req.url.startsWith('/api')) {
    const p = http.request(API + req.url, { method: req.method, headers: req.headers }, pr => {
      res.writeHead(pr.statusCode, pr.headers);
      pr.pipe(res);
    });
    p.on('error', () => { res.writeHead(502); res.end('Bad Gateway'); });
    req.pipe(p);
  } else serveStatic(req, res);
}).listen(3001, '127.0.0.1', () => {
  console.log('Frontend: http://localhost:3001 (API -> ' + API + ')');
});