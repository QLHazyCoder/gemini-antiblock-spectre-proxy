package handlers

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strconv"
    "time"

    "gemini-antiblock/logger"
    "gemini-antiblock/metrics"
)

// LogsJSONHandler returns the current snapshot for UI polling.
func LogsJSONHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    limit := 0
    if s := r.URL.Query().Get("limit"); s != "" {
        if n, err := strconv.Atoi(s); err == nil && n > 0 {
            limit = n
        }
    }
    snap := metrics.GetSnapshot(limit)
    // simple JSON marshal via fmt.Fprintf + Marshal
    // but we can rely on standard encoding/json by using http.ServeContent; here keep simple
    // Use logger to debug request
    _ = snap
    // Use json.NewEncoder to avoid extra allocations
    // Ensure no caching
    if err := json.NewEncoder(w).Encode(snap); err != nil {
        http.Error(w, "Failed to encode snapshot", http.StatusInternalServerError)
        return
    }
}

// LogsSSEHandler provides realtime events via SSE.
func LogsSSEHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
    w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("X-Accel-Buffering", "no")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
        return
    }

    ch := metrics.Subscribe()
    defer metrics.Unsubscribe(ch)

    // Send a comment to keep the connection alive
    fmt.Fprintf(w, ": heartbeat\n\n")
    flusher.Flush()

    notify := r.Context().Done()
    for {
        select {
        case <-notify:
            return
        case line := <-ch:
            if _, err := w.Write(line); err != nil {
                logger.LogError("SSE write error:", err)
                return
            }
            flusher.Flush()
        case <-time.After(30 * time.Second):
            // periodic heartbeat
            if _, err := w.Write([]byte(": ping\n\n")); err != nil {
                return
            }
            flusher.Flush()
        }
    }
}

// LogsPageHandler serves a simple inline HTML page for viewing logs.
// Keep inline for simplicity and to avoid embedding extra assets.
func LogsPageHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    io.WriteString(w, logsHTML)
}

// Minimal, self-contained HTML page that polls JSON and listens to SSE.
const logsHTML = `<!DOCTYPE html>
<html lang="zh-Hans">
<head>
<meta charset="UTF-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
<title>Gemini 抗封锁 · 实时监控</title>
<style>
:root{color-scheme:dark light}
*{box-sizing:border-box}
body.antiblock-body{font-family:"HarmonyOS Sans","PingFang SC","Microsoft YaHei",system-ui,-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;background:#0b1220 !important;color:#dce1f2 !important;margin:0 !important;padding:40px 32px;min-height:100vh;width:100% !important;max-width:none !重要;display:flex;justify-content:center}
a{color:inherit}
#antiblock-app{width:100%;max-width:1280px;display:flex;flex-direction:column;gap:24px}
.top-bar{display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:12px}
.top-bar h1{font-size:26px;margin:0;color:#f8fafc}
.subtitle{margin:0;color:#94a3b8;font-size:13px}
.live-indicator{display:flex;align-items:center;gap:8px;font-size:13px;padding:6px 12px;border-radius:999px;background:#111c2f;border:1px solid #1f2a44}
.live-indicator .dot{width:9px;height:9px;border-radius:50%;background:#10b981;box-shadow:0 0 8px rgba(16,185,129,.65);transition:background .3s ease,box-shadow .3s ease}
.live-indicator.paused .dot{background:#f59e0b;box-shadow:0 0 6px rgba(245,158,11,.6)}
.live-indicator.error .dot{background:#ef4444;box-shadow:0 0 6px rgba(239,68,68,.6)}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:16px}
.card{background:#111827;border:1px solid rgba(148,163,184,.16);border-radius:16px;padding:18px;position:relative;overflow:hidden;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center}
.card::after{content:"";position:absolute;inset:0;pointer-events:none;border-radius:inherit;border:1px solid rgba(255,255,255,.04)}
.num{font-size:30px;font-weight:700;color:#f8fafc;margin-bottom:6px}
.label{color:#94a3b8;font-size:13px;display:flex;align-items:center;justify-content:center;gap:6px}
.table-card{padding:0;display:flex;flex-direction:column;background:rgba(17,24,39,.72);border:1px solid rgba(148,163,184,.16);border-radius:18px;overflow:hidden}
.table-header{display:flex;align-items:center;justify-content:space-between;padding:18px 22px;border-bottom:1px solid rgba(148,163,184,.12);color:#94a3b8;font-size:13px}
.refresh-tip{font-size:12px;color:#64748b}
.table-wrap{overflow:auto;max-height:60vh;scrollbar-width:none;-ms-overflow-style:none}
.table-wrap::-webkit-scrollbar{width:0;height:0}
table{width:100%;border-collapse:collapse;table-layout:auto;min-width:900px}
thead th{position:sticky;top:0;background:rgba(15,23,42,.92);padding:13px 16px;font-size:12px;font-weight:600;color:#cbd5f5;text-align:center;z-index:1;white-space:nowrap}
tbody td{padding:12px 16px;font-size:12px;border-bottom:1px solid rgba(148,163,184,.08);vertical-align:middle;color:#e2e8f0}
tbody tr:nth-child(even){background:rgba(15,23,42,.55)}
tbody tr:hover{background:rgba(59,130,246,.14)}
.status{display:inline-flex;align-items:center;gap:6px;font-weight:600}
.status .dot{width:8px;height:8px;border-radius:50%;display:inline-block}
.status.ok .dot{background:#10b981}
.status.fail .dot{background:#ef4444}
.status.pending .dot{background:#f59e0b}
.pill{display:inline-block;padding:4px 10px;border-radius:999px;background:rgba(59,130,246,.18);color:#bfdbfe;font-weight:600;font-size:11px}
.badge{display:inline-flex;align-items:center;padding:2px 8px;border-radius:999px;border:1px solid rgba(148,163,184,.2);min-width:38px;justify-content:center;font-size:11px}
.badge.yes{color:#4ade80;border-color:rgba(74,222,128,.4)}
.badge.no{color:#fca5a5;border-color:rgba(252,165,165,.35)}
.muted{color:#64748b}
.empty{padding:42px 18px;text-align:center;color:#64748b;font-size:13px}
.toast{position:fixed;right:24px;bottom:24px;padding:12px 16px;background:#1f2937;border:1px solid rgba(148,163,184,.2);color:#e2e8f0;border-radius:10px;font-size:13px;box-shadow:0 12px 32px rgba(15,23,42,.4);opacity:0;pointer-events:none;transition:opacity .3s ease,transform .3s ease;transform:translateY(12px)}
.toast.show{opacity:1;transform:translateY(0)}
.col-path{min-width:220px;max-width:380px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.col-result{min-width:120px;max-width:180px}
.result-cell{white-space:nowrap}
.result-text{max-width:220px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.detail-btn{background:none;border:none;color:#93c5fd;cursor:pointer;font-size:12px;padding:0;display:inline-flex;align-items:center;gap:6px;font-weight:600}
.detail-btn:hover{text-decoration:underline}
.modal{position:fixed;inset:0;display:none;align-items:center;justify-content:center;background:rgba(15,23,42,.65);z-index:50;padding:24px}
.modal.show{display:flex}
.modal-dialog{background:#0f172a;border:1px solid rgba(148,163,184,.25);border-radius:14px;max-width:700px;width:100%;max-height:80vh;display:flex;flex-direction:column}
.modal-header{display:flex;justify-content:space-between;align-items:center;padding:16px 20px;border-bottom:1px solid rgba(148,163,184,.14);color:#e2e8f0;font-weight:600}
.modal-content{padding:18px 20px;font-family:Menlo,Consolas,monospace;font-size:12px;color:#f8fafc;white-space:pre-wrap;word-break:break-word;overflow:auto}
.modal-close{background:none;border:none;color:#cbd5f5;cursor:pointer;font-size:16px}
@media (max-width:640px){body.antiblock-body{padding:16px}.table-header{flex-direction:column;align-items:flex-start;gap:6px}.table-wrap{max-height:55vh}.top-bar h1{font-size:20px}}
</style>
</head>
<body class="antiblock-body">
<div id="antiblock-app">
  <header class="top-bar">
    <div>
      <h1>Gemini 抗封锁 · 实时日志面板</h1>
      <p class="subtitle">监控代理流量、失败与重试情况，支持最近 200 条记录回溯</p>
    </div>
    <div class="live-indicator" id="sse-indicator"><span class="dot"></span><span class="label">实时连接中</span></div>
  </header>
  <section class="grid">
    <div class="card"><div class="num" id="total">-</div><div class="label">累计请求</div></div>
    <div class="card"><div class="num" id="success">-</div><div class="label">成功次数</div></div>
    <div class="card"><div class="num" id="errors">-</div><div class="label">失败次数</div></div>
    <div class="card"><div class="num" id="retries">-</div><div class="label">触发重试</div></div>
    <div class="card"><div class="num" id="successRate">-</div><div class="label">成功率</div></div>
  </section>
  <section class="card table-card">
    <div class="table-header">
      <span>最近 200 条请求记录</span>
      <span class="refresh-tip" id="refresh-tip">正在加载数据…</span>
    </div>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>时间</th>
            <th>模型</th>
            <th>请求方法</th>
            <th class="col-path">请求路径</th>
            <th>是否流式</th>
            <th>抗断流</th>
            <th>HTTP 状态</th>
            <th>重试次数</th>
            <th>耗时 (ms)</th>
            <th class="col-result">结果</th>
            <th>客户端来源</th>
          </tr>
        </thead>
        <tbody id="rows"></tbody>
      </table>
      <div class="empty" id="empty-state" style="display:none">暂无数据，等待新请求…</div>
    </div>
  </section>
</div>
<div class="toast" id="toast"></div>
<div class="modal" id="detail-modal" style="display:none">
  <div class="modal-dialog">
    <div class="modal-header">
      <span>错误详情</span>
      <button class="modal-close" id="modal-close">×</button>
    </div>
    <pre class="modal-content" id="modal-body"></pre>
  </div>
</div>
<script>
const $ = sel => document.querySelector(sel);
const rows = $('#rows');
const emptyState = $('#empty-state');
const indicator = $('#sse-indicator');
const refreshTip = $('#refresh-tip');
const toast = $('#toast');
const detailModal = $('#detail-modal');
const modalBody = $('#modal-body');
const modalClose = $('#modal-close');

const fmtTs = (ts, allowZero = false) => {
  if (!ts) return '—';
  try {
    const d = new Date(ts);
    if (!allowZero && d.getFullYear() <= 1) {
      return '—';
    }
    return d.toLocaleString('zh-CN', { hour12: false });
  } catch (e) {
    return ts;
  }
};

const fmtPercent = (num) => Number.isFinite(num) ? num.toFixed(1) + '%' : '-';

const escapeHTML = (str) => str.replace(/[&<>"']/g, (c) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;','\'':'&#39;'}[c]));

const sanitizeError = (text) => {
  if (!text) return '';
  return text.replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F-\x9F]/g, '').trim();
};

const openModal = (content) => {
  if (!detailModal) return;
  modalBody.textContent = content;
  detailModal.style.display = 'flex';
  detailModal.classList.add('show');
};

const closeModal = () => {
  if (!detailModal) return;
  detailModal.classList.remove('show');
  detailModal.style.display = 'none';
};

if (modalClose) {
  modalClose.addEventListener('click', closeModal);
}
if (detailModal) {
  detailModal.addEventListener('click', (ev) => {
    if (ev.target === detailModal) {
      closeModal();
    }
  });
}

const showToast = (msg) => {
  if (!toast) return;
  toast.textContent = msg;
  toast.classList.add('show');
  setTimeout(() => toast.classList.remove('show'), 2600);
};

const setIndicator = (state) => {
  if (!indicator) return;
  indicator.classList.remove('paused','error');
  const label = indicator.querySelector('.label');
  if (!label) return;
  if (state === 'paused') {
    indicator.classList.add('paused');
    label.textContent = '连接已中断，等待重试…';
  } else if (state === 'error') {
    indicator.classList.add('error');
    label.textContent = '连接失败，请检查服务器';
  } else {
    label.textContent = '实时连接中';
  }
};

const buildResultCell = (entry) => {
  if (entry.success) {
    return '<span class="status ok"><span class="dot"></span>成功</span>';
  }

  const cleanError = sanitizeError(entry.error) || (entry.status ? 'HTTP ' + entry.status + ' 错误' : '无更多细节');
  const encoded = encodeURIComponent(cleanError);
  return '<span class="status fail"><span class="dot"></span><button class="detail-btn" data-error="' + encoded + '">查看详情</button></span>';
};

const buildRow = (entry) => {
  const tr = document.createElement('tr');
  let html = '';
  html += '<td>' + fmtTs(entry.timestamp) + '</td>';
  html += '<td>' + (entry.model ? '<span class="pill">' + entry.model + '</span>' : '<span class="muted">—</span>') + '</td>';
  html += '<td>' + (entry.method || '<span class="muted">—</span>') + '</td>';
  const pathText = entry.path || '';
  const safePath = escapeHTML(pathText);
  html += '<td class="col-path" title="' + safePath + '">' + (pathText ? safePath : '<span class="muted">—</span>') + '</td>';
  html += '<td>' + (entry.streaming ? '<span class="badge yes">是</span>' : '<span class="badge no">否</span>') + '</td>';
  html += '<td>' + (entry.antiblockEnabled ? '<span class="badge yes">是</span>' : '<span class="badge no">否</span>') + '</td>';
  html += '<td>' + (entry.status || '<span class="muted">—</span>') + '</td>';
  html += '<td>' + (entry.retries ?? 0) + '</td>';
  html += '<td>' + (entry.durationMs ?? 0) + '</td>';
  html += '<td class="result-cell">' + buildResultCell(entry) + '</td>';
  html += '<td>' + (entry.clientIp || '<span class="muted">—</span>') + '</td>';
  tr.innerHTML = html;
  return tr;
};

const renderSnapshot = (snapshot) => {
  const stats = snapshot.stats || {};
  $('#total').textContent = stats.totalRequests ?? '-';
  $('#success').textContent = stats.successCount ?? '-';
  $('#errors').textContent = stats.errorCount ?? '-';
  $('#retries').textContent = stats.retryCount ?? '-';

  const total = stats.totalRequests || 0;
  const success = stats.successCount || 0;
  $('#successRate').textContent = total > 0 ? fmtPercent((success / total) * 100) : '-';

  rows.innerHTML = '';
  const logs = snapshot.logs || [];
  logs.forEach(entry => rows.appendChild(buildRow(entry)));
  emptyState.style.display = logs.length ? 'none' : 'block';

  if (logs.length) {
    const validDurations = logs.filter(entry => typeof entry.durationMs === 'number' && entry.durationMs >= 0);
    if (validDurations.length) {
      const avg = validDurations.reduce((acc, cur) => acc + cur.durationMs, 0) / validDurations.length;
      refreshTip.textContent = '平均耗时 ' + avg.toFixed(0) + ' ms · 上次刷新 ' + fmtTs(Date.now());
    } else {
      refreshTip.textContent = '已加载 ' + logs.length + ' 条记录';
    }
  } else {
    refreshTip.textContent = '等待数据写入…';
  }
};

const loadSnapshot = async () => {
  refreshTip.textContent = '正在加载数据…';
  try {
    const res = await fetch('/logs/antiblock.json?limit=200',{cache:'no-store'});
    if (!res.ok) throw new Error('HTTP ' + res.status);
    const json = await res.json();
    renderSnapshot(json);
    refreshTip.textContent = '最新更新 ' + fmtTs(Date.now());
  } catch (err) {
    refreshTip.textContent = '加载失败，请稍后重试';
    showToast('获取日志失败：' + err.message);
  }
};

loadSnapshot();

let lastReloadAt = Date.now();

const debounceReload = () => {
  const now = Date.now();
  if (now - lastReloadAt > 1500) {
    lastReloadAt = now;
    loadSnapshot();
  }
};

try {
  const ev = new EventSource('/logs/stream');
  ev.onopen = () => setIndicator('live');
  ev.onerror = () => setIndicator('paused');
  ev.onmessage = (m) => {
    try {
      const payload = JSON.parse(m.data);
      if (payload.type === 'finish') {
        debounceReload();
      } else if (payload.type === 'retry') {
        showToast('有请求触发重试…');
        debounceReload();
      }
    } catch (err) {
      console.error('解析 SSE 消息失败', err);
    }
  };
} catch (err) {
  setIndicator('error');
  showToast('初始化实时连接失败：' + err.message);
}

const enforceBodyStyle = () => {
  document.body.classList.add('antiblock-body');
  const style = document.body.style;
  style.setProperty('background', '#0b1220', 'important');
  style.setProperty('color', '#dce1f2', 'important');
  style.setProperty('margin', '0', 'important');
  style.setProperty('padding', '40px 32px', 'important');
  style.setProperty('min-height', '100vh', 'important');
  style.setProperty('width', '100%', 'important');
  style.setProperty('max-width', 'none', 'important');
  style.setProperty('font-family', '"HarmonyOS Sans","PingFang SC","Microsoft YaHei",system-ui,-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif', 'important');
};

enforceBodyStyle();
new MutationObserver(enforceBodyStyle).observe(document.body, { attributes: true, attributeFilter: ['style','class'] });

document.addEventListener('click', (ev) => {
  const btn = ev.target.closest('.detail-btn');
  if (btn) {
    const text = decodeURIComponent(btn.getAttribute('data-error') || '');
    openModal(text || '无更多细节');
  }
});
</script>
</body>
</html>
`
