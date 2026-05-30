# -*- coding: utf-8 -*-
import sys, json
sys.stdout.reconfigure(encoding="utf-8")

# Write the HTML prototype
html = open(r'C:\Users\owen6\repo\learn-helper\docs\design\prototype\index.html', 'w', encoding='utf-8')

html.write('<!DOCTYPE html>')
html.write('<html lang="zh-CN">')
html.write('<head>')
html.write('<meta charset="UTF-8">')
html.write('<meta name="viewport" content="width=device-width, initial-scale=1.0">')
html.write('<title>LLM Wiki - Prototype</title>')
html.write('<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">')
html.write('<style>')
html.write(open(r'C:\Users\owen6\repo\learn-helper\docs\design\prototype\style.css', 'r', encoding='utf-8').read())
html.write('</style></head><body>')

# Navbar
html.write('<nav class="navbar"><div class="navbar-logo">'
  '<svg viewBox="0 0 20 20" fill="currentColor"><path d="M10 2a8 8 0 100 16 8 8 0 000-16zM8 5a1 1 0 112 0v5a1 1 0 11-2 0V5zm1 10a1 1 0 100-2 1 1 0 000 2z"/></svg>'
  'LLM Wiki</div>'
  '<div class="nav-tabs"><div class="nav-tab active" data-view="library" onclick="switchView(\'library\')">🧠 知识库</div>'
  '<div class="nav-tab" data-view="browse" onclick="switchView(\'browse\')">📖 浏览</div>'
  '<div class="nav-tab" data-view="dashboard" onclick="switchView(\'dashboard\')">📊 仪表盘</div></div>'
  '<div class="navbar-right"><div class="navbar-icon">⚙️</div></div></nav>')

# Library view
html.write('<div class="view library-view active" id="view-library">')

# Left panel
html.write('<div class="panel panel-left" id="leftPanel">'
  '<div class="panel-header"><h3>🌳 知识库</h3>'
  '<div class="panel-header-actions">'
  '<div class="panel-header-btn">🔍</div>'
  '<div class="panel-header-btn">＋</div>'
  '<div class="panel-header-btn" onclick="togglePanel(\'left\')">◀</div></div></div>'
  '<div class="tree-search"><span class="tree-search-icon">🔍</span><input placeholder="搜索知识树..."></div>'
  '<div class="tree-container" id="knowledgeTree"></div></div>')

html.write('<div class="resize-handle" data-panel="left" onmousedown="startResize(event,\'left\')"></div>')

# Middle panel
html.write('<div class="panel panel-middle" id="middlePanel">'
  '<div class="panel-header"><h3>💬 AI 助手</h3>'
  '<div class="panel-header-actions"><div class="panel-header-btn">🗑️</div></div></div>'
  '<div class="chat-messages" id="chatMessages"></div>'
  '<div class="chat-input-area">'
  '<div class="chat-input-row">'
  '<div class="chat-tools"><div class="chat-tool-btn">📎</div></div>'
  '<textarea rows="1" placeholder="输入消息... (Enter 发送, Shift+Enter 换行)" onkeydown="if(event.key===\'Enter\'&&!event.shiftKey){event.preventDefault();sendChat()}"></textarea>'
  '<button class="chat-send-btn" onclick="sendChat()">➤</button></div></div></div>')

html.write('<div class="resize-handle" data-panel="middle" onmousedown="startResize(event,\'middle\')"></div>')

# Right panel
html.write('<div class="panel panel-right" id="rightPanel">'
  '<div class="panel-header"><h3>📄 页面</h3>'
  '<div class="panel-header-actions"><div class="panel-header-btn">⭐</div><div class="panel-header-btn">⋮</div>'
  '<div class="panel-header-btn" onclick="togglePanel(\'right\')">▶</div></div></div>'
  '<div class="page-browser" id="pageBrowser"></div></div></div>')

# Dashboard view
html.write('<div class="view dashboard" id="view-dashboard">'
  '<h2>📊 仪表盘</h2>'
  '<div class="stats-row">'
  '<div class="stat-card"><div class="value">28</div><div class="label">总页面</div></div>'
  '<div class="stat-card"><div class="value">64%</div><div class="label">内容覆盖度</div></div>'
  '<div class="stat-card"><div class="value">5</div><div class="label">待填充</div></div>'
  '<div class="stat-card"><div class="value">3</div><div class="label">知识领域</div></div></div>'
  '<div class="section-title">📈 领域覆盖</div>'
  '<div class="coverage-list">'
  '<div class="coverage-item"><span class="coverage-name">Go</span><div class="coverage-bar"><div class="coverage-fill" style="width:67%"></div></div><span class="coverage-pct">8/12</span></div>'
  '<div class="coverage-item"><span class="coverage-name">Redis</span><div class="coverage-bar"><div class="coverage-fill" style="width:62%"></div></div><span class="coverage-pct">5/8</span></div>'
  '<div class="coverage-item"><span class="coverage-name">系统设计</span><div class="coverage-bar"><div class="coverage-fill" style="width:50%"></div></div><span class="coverage-pct">3/6</span></div></div>'
  '<div class="section-title">🕐 最近活动</div>'
  '<div class="activity-list">'
  '<div class="activity-item"><div class="activity-dot"></div>📝 更新 goroutine<span class="activity-time">2 小时前</span></div>'
  '<div class="activity-item"><div class="activity-dot"></div>➕ 创建 GMP 模型<span class="activity-time">2 小时前</span></div>'
  '<div class="activity-item"><div class="activity-dot"></div>📝 更新 Channel<span class="activity-time">昨天</span></div>'
  '<div class="activity-item"><div class="activity-dot"></div>📝 更新 Go 基础语法<span class="activity-time">3 天前</span></div>'
  '<div class="activity-item"><div class="activity-dot"></div>➕ 创建 Redis 数据结构<span class="activity-time">5 天前</span></div></div></div>')

# Browse view
html.write('<div class="view browse-view" id="view-browse">'
  '<div class="browse-tree"><div class="panel-header"><h3>🌳 知识库</h3></div>'
  '<div class="tree-container" id="browseTree"></div></div>'
  '<div class="browse-content"><div class="page-browser" id="browseContent"></div></div></div>')

# Script
html.write('<script>')
html.write(open(r'C:\Users\owen6\repo\learn-helper\docs\design\prototype\script.js', 'r', encoding='utf-8').read())
html.write('</script></body></html>')

html.close()
print("HTML written")
