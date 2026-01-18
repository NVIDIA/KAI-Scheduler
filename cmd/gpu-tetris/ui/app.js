let state = {
  selectedDomainId: 'root',
  viz: null,
};

function el(tag, attrs = {}, children = []) {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (k === 'class') node.className = v;
    else if (k.startsWith('on') && typeof v === 'function') node.addEventListener(k.slice(2), v);
    else node.setAttribute(k, v);
  }
  for (const c of children) node.appendChild(typeof c === 'string' ? document.createTextNode(c) : c);
  return node;
}

function hashColor(key) {
  // Deterministic color from key, using HSL.
  let h = 2166136261;
  for (let i = 0; i < key.length; i++) {
    h ^= key.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  const hue = Math.abs(h) % 360;
  return `hsl(${hue} 65% 70%)`;
}

async function fetchViz() {
  const resp = await fetch('/viz.json', { cache: 'no-store' });
  if (!resp.ok) {
    const msg = await resp.text();
    throw new Error(msg);
  }
  return resp.json();
}

function renderMeta(viz) {
  const meta = document.getElementById('meta');
  meta.textContent = `Generated: ${viz.generatedAt}`;
}

function renderPending(viz) {
  const root = document.getElementById('pending');
  if (!root) return;
  root.innerHTML = '';

  const pending = (viz.pending || []).slice();
  if (pending.length === 0) {
    root.appendChild(el('div', { class: 'pending-empty' }, ['No pending GPU pods.']));
    return;
  }

  for (const p of pending) {
    const title = `${p.namespace}/${p.pod}`;
    const subtitleParts = [];
    if (p.request) subtitleParts.push(p.request);
    if (p.queue) subtitleParts.push(`queue: ${p.queue}`);
    if (p.reason) subtitleParts.push(p.reason);

    const accentKey = String(p.queue || title);
    const accent = hashColor(accentKey);

    root.appendChild(el('div', { class: 'pending-item', title, style: `border-left-color:${accent};` }, [
      el('div', { class: 'pending-title' }, [title]),
      el('div', { class: 'pending-sub' }, [subtitleParts.join(' • ')])
    ]));
  }
}

async function createPod(payload) {
  const resp = await fetch('/api/pods', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const text = await resp.text();
  if (!resp.ok) throw new Error(text || resp.statusText);
  return JSON.parse(text);
}

async function deleteTetrisPods() {
  const resp = await fetch('/api/pods', {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
  });
  const text = await resp.text();
  if (!resp.ok) throw new Error(text || resp.statusText);
  return JSON.parse(text);
}

function setBusy(btn, busy) {
  if (!btn) return;
  btn.disabled = !!busy;
}

function setStatus(elm, message, details) {
  if (!elm) return;
  elm.innerHTML = '';
  elm.appendChild(document.createTextNode(message || ''));
  if (details && details.length) {
    const ul = document.createElement('ul');
    ul.className = 'status-details';
    for (const d of details.slice(0, 6)) {
      const li = document.createElement('li');
      li.textContent = String(d);
      ul.appendChild(li);
    }
    if (details.length > 6) {
      const li = document.createElement('li');
      li.textContent = `…and ${details.length - 6} more`;
      ul.appendChild(li);
    }
    elm.appendChild(ul);
  }
}

function setupCreatePodModeUI(form) {
  if (!form) return;
  const modeEl = form.querySelector('select[name="mode"]');
  if (!modeEl) return;

  function showField(name, show) {
    const input = form.querySelector(`[name="${name}"]`);
    if (!input) return;
    const label = input.closest('label') || input.parentElement;
    if (label) label.style.display = show ? '' : 'none';
    input.disabled = !show;
  }

  function apply() {
    const mode = String(modeEl.value || 'whole');
    showField('gpuCount', mode === 'whole');
    showField('gpuFraction', mode === 'fraction');
    showField('fractionNumDevices', mode === 'fraction');
    showField('gpuMemoryMiB', mode === 'memory');
  }

  modeEl.addEventListener('change', apply);
  apply();
}

async function createTopology(payload) {
  const resp = await fetch('/api/topology', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const text = await resp.text();
  if (!resp.ok) throw new Error(text || resp.statusText);
  return JSON.parse(text);
}

async function createQueue(payload) {
  const resp = await fetch('/api/queues', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const text = await resp.text();
  if (!resp.ok) throw new Error(text || resp.statusText);
  return JSON.parse(text);
}

function setupCreateResourceForm() {
  const form = document.getElementById('createResource');
  const status = document.getElementById('createResourceStatus');
  if (!form || !status) return;

  const typeInput = document.getElementById('resourceTypeInput');
  const tabs = document.getElementById('resourceTypeTabs');
  const podFields = document.getElementById('podFields');
  const queueFields = document.getElementById('queueFields');
  const topologyFields = document.getElementById('topologyFields');

  // Setup GPU mode UI for pod fields
  setupCreatePodModeUI(form);

  // Handle tab switching
  function updateFieldsVisibility() {
    const resourceType = typeInput.value;
    podFields.style.display = resourceType === 'pod' ? '' : 'none';
    queueFields.style.display = resourceType === 'queue' ? '' : 'none';
    topologyFields.style.display = resourceType === 'topology' ? '' : 'none';
  }

  // Tab click handlers
  tabs.addEventListener('click', (e) => {
    const tab = e.target.closest('.tab');
    if (!tab) return;
    const type = tab.dataset.type;
    if (!type) return;

    // Update active state
    tabs.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    tab.classList.add('active');

    // Update hidden input
    typeInput.value = type;
    updateFieldsVisibility();
  });

  updateFieldsVisibility();

  const submitBtn = form.querySelector('button[type="submit"]');

  // Delete tetris pods button
  const deleteBtn = document.getElementById('deleteTetrisPods');
  const deleteStatus = document.getElementById('deleteTetrisPodsStatus');
  if (deleteBtn && deleteStatus) {
    deleteBtn.addEventListener('click', async () => {
      setBusy(deleteBtn, true);
      setStatus(deleteStatus, 'Deleting…');
      try {
        const res = await deleteTetrisPods();
        if (res.errors && res.errors.length) {
          setStatus(deleteStatus, `Deleted ${res.deleted}. Errors: ${res.errors.length}`, res.errors);
        } else {
          setStatus(deleteStatus, `Deleted ${res.deleted} pod(s).`);
        }
        await refresh();
      } catch (err) {
        setStatus(deleteStatus, `Error: ${err.message}`);
      } finally {
        setBusy(deleteBtn, false);
      }
    });
  }

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    setBusy(submitBtn, true);

    const fd = new FormData(form);
    const resourceType = fd.get('resourceType');

    try {
      if (resourceType === 'pod') {
        setStatus(status, 'Creating pod…');
        const mode = String(fd.get('mode') || 'whole');
        const payload = {
          namespace: String(fd.get('namespace') || ''),
          name: String(fd.get('podName') || ''),
          queue: String(fd.get('queue') || ''),
          mode,
          gpuCount: Number(fd.get('gpuCount') || 0),
          gpuFraction: Number(fd.get('gpuFraction') || 0),
          fractionNumDevices: Number(fd.get('fractionNumDevices') || 0),
          gpuMemoryMiB: Number(fd.get('gpuMemoryMiB') || 0),
          image: String(fd.get('image') || ''),
        };
        const res = await createPod(payload);
        setStatus(status, `Created pod ${res.namespace}/${res.name}. Waiting for scheduling…`);
      } else if (resourceType === 'queue') {
        setStatus(status, 'Creating queue…');
        const priorityStr = fd.get('priority');
        const gpuQuotaStr = fd.get('gpuQuota');
        const payload = {
          name: String(fd.get('queueName') || ''),
          displayName: String(fd.get('displayName') || ''),
          parentQueue: String(fd.get('parentQueue') || ''),
          priority: priorityStr ? Number(priorityStr) : null,
          gpuQuota: gpuQuotaStr ? Number(gpuQuotaStr) : 0,
        };
        const res = await createQueue(payload);
        setStatus(status, `Created queue "${res.name}".`);
      } else if (resourceType === 'topology') {
        setStatus(status, 'Creating topology…');
        const name = String(fd.get('topologyName') || '').trim();
        const levelsRaw = String(fd.get('levels') || '');
        const levels = levelsRaw.split(',').map(s => s.trim()).filter(Boolean);

        const assignmentsRaw = String(fd.get('assignments') || '');
        const assignments = [];
        for (const line of assignmentsRaw.split('\n')) {
          const trimmed = line.trim();
          if (!trimmed) continue;
          const parts = trimmed.split(/\s+/);
          const node = parts[0];
          const values = parts.slice(1);
          assignments.push({ node, values });
        }

        const res = await createTopology({ name, levels, assignments });
        if (res.errors && res.errors.length) {
          setStatus(status, `Created ${res.topologyName}. Patched ${res.patchedNodes}. Errors: ${res.errors.length}`, res.errors);
        } else {
          setStatus(status, `Created ${res.topologyName}. Patched ${res.patchedNodes} node(s).`);
        }
      }
      await refresh();
    } catch (err) {
      setStatus(status, `Error: ${err.message}`);
    } finally {
      setBusy(submitBtn, false);
    }
  });
}

function renderTopology(viz) {
  const root = document.getElementById('topology');
  root.innerHTML = '';

  function renderNode(n) {
    const dot = el('span', { class: 'dot', style: `background:${hashColor(n.id)};` });
    const btn = el('button', {
      class: `tree-button ${state.selectedDomainId === n.id ? 'active' : ''}`,
      onclick: () => {
        state.selectedDomainId = n.id;
        renderTopology(viz);
        renderNodes(viz);
      },
      title: `${n.nodeNames.length} node(s)`
    }, [dot, `${n.name} (${n.nodeNames.length})`]);

    const item = el('li', { class: 'tree-item' }, [btn]);

    if (n.children && n.children.length) {
      const ul = el('ul', { class: 'tree' });
      for (const c of n.children) ul.appendChild(renderNode(c));
      item.appendChild(ul);
    }
    return item;
  }

  const ul = el('ul', { class: 'tree' });
  ul.appendChild(renderNode(viz.topology));
  root.appendChild(ul);
}

function renderQueues(viz) {
  const root = document.getElementById('queues');
  if (!root) return;
  root.innerHTML = '';

  const queues = viz.queues || [];
  if (queues.length === 0) {
    root.appendChild(el('div', { class: 'pending-empty' }, ['No queues found.']));
    return;
  }

  function formatGpu(val) {
    if (val === 0) return '0';
    if (val >= 1) return val.toFixed(1);
    return val.toFixed(2);
  }

  function renderQueueNode(q) {
    const dot = el('span', { class: 'dot', style: `background:${hashColor(q.name)};` });
    const allocated = formatGpu(q.allocatedGpu);
    const fairShare = formatGpu(q.fairShareGpu);
    const requested = formatGpu(q.requestedGpu);
    const displayName = q.displayName || q.name;

    // Show allocated/fairShare/requested, use '-' if fairShare is 0 (not configured)
    const fairShareStr = q.fairShareGpu > 0 ? fairShare : '-';
    const badge = `${allocated}/${fairShareStr}/${requested}`;

    const btn = el('button', {
      class: 'tree-button',
      title: `${displayName}\nAllocated: ${allocated} GPU\nFairShare: ${fairShareStr} GPU\nRequested: ${requested} GPU\nPriority: ${q.priority}`
    }, [dot, el('span', {}, [displayName]), el('span', { class: 'queue-gpu-badge' }, [badge])]);

    const item = el('li', { class: 'tree-item' }, [btn]);

    if (q.children && q.children.length) {
      const ul = el('ul', { class: 'tree' });
      for (const c of q.children) ul.appendChild(renderQueueNode(c));
      item.appendChild(ul);
    }
    return item;
  }

  const ul = el('ul', { class: 'tree' });
  for (const q of queues) {
    ul.appendChild(renderQueueNode(q));
  }
  root.appendChild(ul);
}

function updateQueueDropdown(viz) {
  const select = document.getElementById('queueSelect');
  if (!select) return;

  const currentValue = select.value;
  const queues = viz.queues || [];

  // Flatten queue tree with indentation to show hierarchy
  function collectQueues(queueList, depth = 0) {
    const result = [];
    for (const q of queueList) {
      const indent = '\u00A0\u00A0'.repeat(depth); // Non-breaking spaces for indentation
      const displayName = q.displayName || q.name;
      result.push({ name: q.name, label: indent + displayName, depth });
      if (q.children && q.children.length > 0) {
        result.push(...collectQueues(q.children, depth + 1));
      }
    }
    return result;
  }

  const allQueues = collectQueues(queues);

  // Clear existing options except the first placeholder
  while (select.options.length > 1) {
    select.remove(1);
  }

  // Add queue options
  for (const q of allQueues) {
    const opt = document.createElement('option');
    opt.value = q.name;
    opt.textContent = q.label;
    select.appendChild(opt);
  }

  // Restore previous selection if it still exists
  if (currentValue && Array.from(select.options).some(o => o.value === currentValue)) {
    select.value = currentValue;
  }
}

function updateParentQueueDropdown(viz) {
  const select = document.getElementById('parentQueueSelect');
  if (!select) return;

  const currentValue = select.value;
  const queues = viz.queues || [];

  // Flatten queue tree with indentation to show hierarchy
  function collectQueues(queueList, depth = 0) {
    const result = [];
    for (const q of queueList) {
      const indent = '\u00A0\u00A0'.repeat(depth); // Non-breaking spaces for indentation
      const displayName = q.displayName || q.name;
      result.push({ name: q.name, label: indent + displayName, depth });
      if (q.children && q.children.length > 0) {
        result.push(...collectQueues(q.children, depth + 1));
      }
    }
    return result;
  }

  const allQueues = collectQueues(queues);

  // Clear existing options except the first placeholder
  while (select.options.length > 1) {
    select.remove(1);
  }

  // Add queue options
  for (const q of allQueues) {
    const opt = document.createElement('option');
    opt.value = q.name;
    opt.textContent = q.label;
    select.appendChild(opt);
  }

  // Restore previous selection if it still exists
  if (currentValue && Array.from(select.options).some(o => o.value === currentValue)) {
    select.value = currentValue;
  }
}

function domainById(root, id) {
  if (root.id === id) return root;
  for (const c of (root.children || [])) {
    const f = domainById(c, id);
    if (f) return f;
  }
  return null;
}

function renderNodes(viz) {
  const container = document.getElementById('nodes');
  if (!container) return;

  const domain = domainById(viz.topology, state.selectedDomainId) || viz.topology;
  const allowed = new Set(domain.nodeNames);

  const blocksByNode = new Map();
  for (const b of viz.blocks) {
    if (!allowed.has(b.nodeName)) continue;
    if (!blocksByNode.has(b.nodeName)) blocksByNode.set(b.nodeName, []);
    blocksByNode.get(b.nodeName).push(b);
  }

  const nodes = viz.nodes.filter(n => allowed.has(n.name));

  // Track which node cards should remain.
  const keepNodes = new Set(nodes.map(n => n.name));

  // Remove node cards that are no longer visible.
  for (const child of Array.from(container.children)) {
    const nodeName = child.getAttribute && child.getAttribute('data-node');
    if (nodeName && !keepNodes.has(nodeName)) {
      child.remove();
    }
  }

  // Ensure a stable ordering matching the current nodes list.
  const desiredOrder = nodes.map(n => n.name);
  for (const nodeName of desiredOrder) {
    const existing = container.querySelector(`.node[data-node="${CSS.escape(nodeName)}"]`);
    if (existing) continue;
    const nodeEl = el('div', { class: 'node', 'data-node': nodeName }, [
      el('div', { class: 'node-title' }, ['']),
      el('div', { class: 'board' })
    ]);
    container.appendChild(nodeEl);
  }
  // Reorder DOM to match desired order.
  for (const nodeName of desiredOrder) {
    const nodeEl = container.querySelector(`.node[data-node="${CSS.escape(nodeName)}"]`);
    if (nodeEl) container.appendChild(nodeEl);
  }

  for (const node of nodes) {
    const nodeEl = container.querySelector(`.node[data-node="${CSS.escape(node.name)}"]`);
    if (!nodeEl) continue;
    const titleEl = nodeEl.querySelector('.node-title');
    if (titleEl) titleEl.textContent = `${node.name} — ${node.gpuCount} GPU(s)`;

    const board = nodeEl.querySelector('.board');
    if (!board) continue;

    // If GPU count changed, rebuild columns.
    const existingCols = Array.from(board.querySelectorAll('.col'));
    if (existingCols.length !== node.gpuCount) {
      board.innerHTML = '';
      for (let gi = 0; gi < node.gpuCount; gi++) {
        board.appendChild(el('div', { class: 'col', title: `GPU ${gi}`, 'data-gpu': String(gi) }));
      }
    }

    const nodeBlocks = (blocksByNode.get(node.name) || []).slice();
    const byGpu = Array.from({ length: node.gpuCount }, () => []);
    for (const b of nodeBlocks) {
      const idx = Math.max(0, Math.min(node.gpuCount - 1, b.gpuIndex));
      byGpu[idx].push(b);
    }

    for (let gi = 0; gi < node.gpuCount; gi++) {
      const col = board.querySelector(`.col[data-gpu="${gi}"]`) || board.querySelectorAll('.col')[gi];
      if (!col) continue;

      const desired = [];
      let offset = 0;
      for (const b of byGpu[gi]) {
        const h = Math.max(0.05, Math.min(1, b.height));
        const px = Math.round(h * 180);
        desired.push({
          id: b.id,
          pod: b.pod,
          namespace: b.namespace,
          colorKey: b.colorKey,
          heightPx: px,
          bottomPx: offset,
          title: `${b.namespace}/${b.pod} (gpu ${gi}, ${b.height.toFixed(2)} gpu)`
        });
        offset += px;
        if (offset >= 180) break;
      }

      const existingById = new Map();
      for (const blk of Array.from(col.querySelectorAll('.block[data-id]'))) {
        existingById.set(blk.getAttribute('data-id'), blk);
      }

      const keep = new Set(desired.map(d => d.id));
      for (const [id, blk] of existingById.entries()) {
        if (!keep.has(id)) {
          blk.classList.add('block-exit');
          setTimeout(() => blk.remove(), 220);
        }
      }

      for (const d of desired) {
        let blk = existingById.get(d.id);
        if (!blk) {
          blk = el('div', {
            class: 'block block-enter',
            'data-id': d.id,
          });
          blk.appendChild(el('div', { class: 'block-label' }, [d.pod]));
          col.appendChild(blk);
          // Ensure the entry animation triggers.
          requestAnimationFrame(() => blk.classList.remove('block-enter'));
        } else {
          const label = blk.querySelector('.block-label');
          if (label) label.textContent = d.pod;
        }

        blk.title = d.title;
        blk.style.bottom = `${d.bottomPx}px`;
        blk.style.height = `${d.heightPx}px`;
        blk.style.background = hashColor(d.colorKey);
      }
    }
  }

  if (nodes.length === 0) {
    container.innerHTML = '';
    container.appendChild(el('div', {}, ['No GPU nodes in this topology domain.']));
  }
}

async function refresh() {
  try {
    const viz = await fetchViz();
    state.viz = viz;
    renderMeta(viz);

    // Keep selection stable if possible.
    if (!domainById(viz.topology, state.selectedDomainId)) {
      state.selectedDomainId = 'root';
    }

    renderTopology(viz);
    renderQueues(viz);
    updateQueueDropdown(viz);
    updateParentQueueDropdown(viz);
    renderPending(viz);
    renderNodes(viz);
  } catch (e) {
    document.getElementById('meta').textContent = `Error: ${e.message}`;
  }
}

refresh();
setupCreateResourceForm();
setInterval(refresh, 5000);
