// Tree Data
const treeData = [
  { id:"go", title:"Go", count:5, state:"filled", children:[
    { id:"goroutine", title:"goroutine", count:3, state:"filled", children:[
      { id:"gmp", title:"GMP 模型", count:0, state:"partial" },
      { id:"channel", title:"Channel", count:0, state:"filled" },
      { id:"sync", title:"sync 原语", count:0, state:"empty" }
    ]},
    { id:"datatypes", title:"数据类型", count:0, state:"partial" },
    { id:"patterns", title:"并发模式", count:0, state:"empty" }
  ]},
  { id:"redis", title:"Redis", count:2, state:"filled", children:[
    { id:"ds", title:"数据结构", count:0, state:"filled" },
    { id:"persist", title:"持久化", count:0, state:"partial" }
  ]},
  { id:"sysdesign", title:"系统设计", count:2, state:"partial", children:[
    { id:"cap", title:"CAP 理论", count:0, state:"filled" },
    { id:"consist", title:"一致性算法", count:0, state:"empty" }
  ]},
  { id:"overview", title:"📊 概览", count:0, state:"filled" }
];

const pages = {
  goroutine: {
    title:"goroutine", subtitle:"goroutine 与并发", breadcrumb:["Go","goroutine"],
    status:"published", tags:["#concurrency","#basics"], updated:"2026-05-29",
    content:"<h2>基本概念</h2><p>Goroutine 是 Go 语言的轻量级线程，由 Go 运行时管理。它比操作系统线程轻量得多，创建成本极低。</p><pre><code>go func() {\n    fmt.Println(\"hello from goroutine\")\n}()</code></pre><h2>调度模型</h2><p>GMP 模型是 Go 的运行时调度核心：</p><ul><li><b>G</b> (Goroutine) — 用户态协程</li><li><b>M</b> (Machine) — 操作系统线程</li><li><b>P</b> (Processor) — 逻辑处理器</li></ul><h2>通信机制</h2><p>Channel 是 goroutine 之间的通信桥梁：</p><pre><code>ch := make(chan int)\ngo func() { ch <- 42 }()\nvalue := <-ch</code></pre>"
  },
  gmp: {
    title:"GMP 模型", subtitle:"Go 的并发调度模型", breadcrumb:["Go","goroutine","GMP 模型"],
    status:"draft", tags:["#concurrency","#scheduler"], updated:"2026-05-29",
    content:"<p>GMP 模型是 Go 运行时调度器的核心架构。</p><h2>三个角色</h2><ul><li><b>G (Goroutine)</b> — 用户态轻量级线程</li><li><b>M (Machine)</b> — 操作系统线程</li><li><b>P (Processor)</b> — 逻辑处理器</li></ul>"
  },
  channel: {
    title:"Channel", subtitle:"Goroutine 间通信", breadcrumb:["Go","goroutine","Channel"],
    status:"published", tags:["#concurrency","#communication"], updated:"2026-05-28",
    content:"<p>Channel 是 Go 中 goroutine 之间通信的主要方式。</p><pre><code>ch := make(chan int, 2)\nch <- 1\nch <- 2\nfmt.Println(<-ch)</code></pre>"
  },
  sync: {
    title:"sync 原语", subtitle:"", breadcrumb:["Go","goroutine","sync 原语"],
    status:"empty", tags:[], updated:"2026-05-28", content:""
  },
  datatypes: {
    title:"数据类型", subtitle:"", breadcrumb:["Go","数据类型"],
    status:"draft", tags:["#basics"], updated:"2026-05-27",
    content:"<p>Go 语言提供了丰富的数据类型。</p><ul><li>bool</li><li>string</li><li>int / int8 / ... / int64</li><li>float32 / float64</li></ul>"
  },
  patterns: {
    title:"并发模式", subtitle:"", breadcrumb:["Go","并发模式"],
    status:"empty", tags:[], updated:"2026-05-27", content:""
  },
  ds: {
    title:"数据结构", subtitle:"", breadcrumb:["Redis","数据结构"],
    status:"published", tags:["#redis","#data-structures"], updated:"2026-05-26",
    content:"<p>Redis 提供了多种数据结构：</p><ul><li>String — 字符串</li><li>List — 列表</li><li>Set — 集合</li><li>Hash — 哈希</li><li>ZSet — 有序集合</li></ul>"
  },
  persist: {
    title:"持久化", subtitle:"", breadcrumb:["Redis","持久化"],
    status:"draft", tags:["#redis","#persistence"], updated:"2026-05-26",
    content:"<p>Redis 支持 RDB 和 AOF 两种持久化方式。</p>"
  },
  cap: {
    title:"CAP 理论", subtitle:"", breadcrumb:["系统设计","CAP 理论"],
    status:"published", tags:["#distributed","#theory"], updated:"2026-05-25",
    content:"<p>CAP 定理指出分布式系统最多同时满足两项。</p>"
  },
  consist: {
    title:"一致性算法", subtitle:"", breadcrumb:["系统设计","一致性算法"],
    status:"empty", tags:[], updated:"2026-05-25", content:""
  },
  overview: {
    title:"知识库概览", subtitle:"AI 自动维护", breadcrumb:["概览"],
    status:"published", tags:["#auto"], updated:"2026-05-29 22:00",
    content:"<h2>统计</h2><p>📄 总页面数: <b>28</b> | ✅ 已填充: <b>18 (64%)</b> | 📝 草稿: <b>5 (18%)</b> | ⬜ 空页面: <b>5 (18%)</b></p><h2>领域分布</h2><ul><li><b>Go</b> — 8 页面 (4 已填充)</li><li><b>Redis</b> — 5 页面 (3 已填充)</li><li><b>系统设计</b> — 4 页面 (2 已填充)</li></ul><h2>最近更新</h2><ul><li>goroutine — 2 小时前</li><li>GMP 模型 — 2 小时前</li><li>Channel — 昨天</li></ul><p><i>AI 自动维护 · 最后更新: 2026-05-29 22:00</i></p>"
  }
};

function renderTree(container, nodes, depth) {
  depth = depth || 0;
  nodes.forEach(n => {
    const hasChildren = n.children && n.children.length > 0;
    const div = document.createElement("div");
    div.className = "tree-node" + (n.id === "goroutine" ? " selected" : "");
    div.dataset.id = n.id;
    div.style.paddingLeft = (8 + depth * 16) + "px";
    div.onclick = () => selectTreeNode(n.id);
    const arrow = document.createElement("span");
    arrow.className = "tree-node-arrow" + (hasChildren ? " expanded" : " hidden");
    arrow.textContent = "▶";
    arrow.onclick = (e) => { e.stopPropagation(); toggleTreeNode(div, arrow); };
    div.appendChild(arrow);
    const dot = document.createElement("span");
    dot.className = "tree-node-dot " + n.state;
    div.appendChild(dot);
    const text = document.createElement("span");
    text.className = "tree-node-text";
    text.textContent = n.title;
    div.appendChild(text);
    if (n.count > 0) {
      const cnt = document.createElement("span");
      cnt.className = "tree-node-count";
      cnt.textContent = "(" + n.count + ")";
      div.appendChild(cnt);
    }
    if (hasChildren) {
      const more = document.createElement("span");
      more.className = "tree-node-more";
      more.textContent = "⋮";
      div.appendChild(more);
    }
    container.appendChild(div);
    if (hasChildren) {
      const ch = document.createElement("div");
      ch.className = "tree-children";
      renderTree(ch, n.children, depth + 1);
      container.appendChild(ch);
    }
  });
}

function toggleTreeNode(node, arrow) {
  const ch = node.nextElementSibling;
  if (ch && ch.classList.contains("tree-children")) {
    const isHidden = ch.style.display === "none";
    ch.style.display = isHidden ? "" : "none";
    arrow.classList.toggle("expanded", isHidden);
  }
}

function selectTreeNode(id) {
  document.querySelectorAll(".tree-node").forEach(n => n.classList.remove("selected"));
  const node = document.querySelector('.tree-node[data-id="' + id + '"]');
  if (node) { node.classList.add("selected"); }
  loadPage(id);
}

function loadPage(id) {
  const page = pages[id];
  const container = document.getElementById("pageBrowser");
  if (!page) {
    container.innerHTML = '<div class="page-content"><p>选择左侧节点查看内容</p></div>';
    return;
  }
  if (page.status === "empty" || !page.content) {
    container.innerHTML = '<div class="empty-state"><div class="empty-state-icon">⬜</div><h3>空页面</h3><p>这个页面还没有内容。<br>在聊天中告诉 AI：<br><b>"展开讲讲 ' + page.title + '"</b></p></div>';
    return;
  }
  const statusMap = { published:"✅ 已发布", draft:"📝 草稿", empty:"⬜ 空页面" };
  const bc = page.breadcrumb.map(b => "<span>" + b + "</span>").join('<span style="color:var(--color-border)"> / </span>');
  const tags = (page.tags || []).map(t => '<span class="tag">' + t + "</span>").join(" ");
  container.innerHTML = '<div class="page-header"><div class="page-breadcrumb">' + bc + '</div><div class="page-title"># ' + page.title + '</div><div class="page-subtitle">' + page.subtitle + '</div><div class="page-meta"><span class="page-status ' + page.status + '">' + statusMap[page.status] + '</span><span>最后更新: ' + page.updated + '</span>' + (tags ? "<span>" + tags + "</span>" : "") + '</div></div><div class="page-content">' + page.content + '</div><div class="page-footer">AI 自动维护 · 第 ' + id + ' 号页面</div>';
  const bc2 = document.getElementById("browseContent");
  if (bc2) bc2.innerHTML = container.innerHTML;
}

function sendChat() {
  const inp = document.querySelector(".chat-input-row textarea");
  const msg = inp.value.trim();
  if (!msg) return;
  inp.value = "";
  inp.style.height = "auto";
  addChatMsg("user", msg);
  const lower = msg.toLowerCase();
  if (lower.includes("go") && (lower.includes("学") || lower.includes("后端"))) {
    setTimeout(() => simulateAI(), 500);
  } else if (lower.includes("hello") || lower.includes("hi") || lower.includes("你好")) {
    setTimeout(() => addChatMsg("ai", "你好！我是你的知识库管家。告诉我你想学什么，我来帮你建立知识库。<br><br>你可以：<br>• 输入 \"我要学...\" 新建知识领域<br>• 输入 \"展开讲...\" 深入某个主题<br>• 拖拽文件让我帮你吸收资料"), 400);
  } else {
    setTimeout(() => addChatMsg("ai", "收到！让我想想... 我已理解你的需求，请确认以下变更计划。"), 400);
    setTimeout(() => addPreview(), 1200);
  }
}

function addChatMsg(type, html) {
  const c = document.getElementById("chatMessages");
  const d = document.createElement("div");
  d.className = "chat-msg " + type;
  d.innerHTML = '<div class="chat-bubble ' + type + '">' + html + "</div>";
  c.appendChild(d);
  c.scrollTop = c.scrollHeight;
}

function addPreview() {
  const c = document.getElementById("chatMessages");
  const d = document.createElement("div");
  d.className = "chat-msg ai";
  d.innerHTML = '<div class="chat-bubble ai"><p>📋 以下是变更预览：</p><div class="preview-card"><h4>📝 待执行变更</h4><div class="preview-item"><span>🟢</span><span>CREATE</span><span style="color:var(--color-text-secondary)">Go 基础语法</span></div><div class="preview-item"><span>🟡</span><span>UPDATE</span><span style="color:var(--color-text-secondary)">goroutine (添加"调度原理"章节)</span></div><div class="preview-item"><span>🟢</span><span>CREATE</span><span style="color:var(--color-text-secondary)">GMP 模型 (子页面)</span></div><div class="preview-actions"><button class="btn btn-secondary" onclick="this.parentElement.innerHTML=\'已发送调整请求\'">✏️ 调整</button><button class="btn btn-danger" onclick="this.parentElement.innerHTML=\'已拒绝\'">❌ 拒绝</button><button class="btn btn-primary" onclick="this.parentElement.innerHTML=\'✅ 已确认，正在写入...\'">✅ 确认</button></div></div></div>';
  c.appendChild(d);
  c.scrollTop = c.scrollHeight;
}

function simulateAI() {
  addChatMsg("ai", '好的！我来为你规划 Go 后端的学习路线：<br><br>📚 <b>大纲：</b><br>• <b>Go 基础语法</b> — 变量、类型、控制流<br>• <b>goroutine 与并发</b> — 协程、channel、GMP<br>• <b>Web 框架</b> — Gin / Echo<br>• <b>数据库操作</b> — GORM、SQL<br>• <b>项目实战</b> — REST API 开发<br><br>共将创建 5 个新页面，是否确认？');
  setTimeout(() => addPreview(), 600);
}

function switchView(name) {
  document.querySelectorAll(".view").forEach(v => v.classList.remove("active"));
  document.querySelectorAll(".nav-tab").forEach(t => t.classList.remove("active"));
  document.getElementById("view-" + name).classList.add("active");
  document.querySelector('.nav-tab[data-view="' + name + '"]').classList.add("active");
  if (name === "browse") {
    const bt = document.getElementById("browseTree");
    bt.innerHTML = "";
    renderTree(bt, treeData, 0);
  }
}

let leftVisible = true, rightVisible = true;
function togglePanel(side) {
  const panel = document.getElementById(side + "Panel");
  const btn = document.getElementById("toggle" + (side === "left" ? "Left" : "Right"));
  if (side === "left") {
    leftVisible = !leftVisible;
    panel.style.display = leftVisible ? "" : "none";
    btn.textContent = leftVisible ? "◀" : "▶";
  } else {
    rightVisible = !rightVisible;
    panel.style.display = rightVisible ? "" : "none";
    btn.textContent = rightVisible ? "▶" : "◀";
  }
}

let isResizing = false, currentResize = null, startX, startWidth;
function startResize(e, panel) {
  isResizing = true;
  currentResize = panel;
  startX = e.clientX;
  const el = document.getElementById(panel + "Panel");
  startWidth = el.offsetWidth;
  document.body.style.cursor = "col-resize";
  document.body.style.userSelect = "none";
}

document.addEventListener("mousemove", e => {
  if (!isResizing) return;
  const el = document.getElementById(currentResize + "Panel");
  let newW;
  if (currentResize === "left") {
    newW = startWidth + (e.clientX - startX);
    newW = Math.max(220, Math.min(400, newW));
  } else {
    newW = startWidth - (e.clientX - startX);
    newW = Math.max(320, Math.min(640, newW));
  }
  el.style.width = newW + "px";
  el.style.flex = "none";
});

document.addEventListener("mouseup", () => {
  isResizing = false;
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
});

renderTree(document.getElementById("knowledgeTree"), treeData, 0);
loadPage("goroutine");

document.querySelector("textarea").addEventListener("input", function() {
  this.style.height = "auto";
  this.style.height = Math.min(this.scrollHeight, 120) + "px";
});
