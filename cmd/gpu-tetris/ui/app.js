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

function setupCreatePodForm() {
  const form = document.getElementById('createPod');
  const status = document.getElementById('createPodStatus');
  if (!form || !status) return;

  setupCreatePodModeUI(form);

  const submitBtn = form.querySelector('button[type="submit"]');

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
    setStatus(status, 'Creating…');

    const fd = new FormData(form);
    const mode = String(fd.get('mode') || 'whole');

    const payload = {
      namespace: String(fd.get('namespace') || ''),
      name: String(fd.get('name') || ''),
      queue: String(fd.get('queue') || ''),
      mode,
      gpuCount: Number(fd.get('gpuCount') || 0),
      gpuFraction: Number(fd.get('gpuFraction') || 0),
      fractionNumDevices: Number(fd.get('fractionNumDevices') || 0),
      gpuMemoryMiB: Number(fd.get('gpuMemoryMiB') || 0),
      image: String(fd.get('image') || ''),
    };

    try {
      const res = await createPod(payload);
      setStatus(status, `Created ${res.namespace}/${res.name}. Waiting for scheduling…`);
      await refresh();
    } catch (err) {
      setStatus(status, `Error: ${err.message}`);
    } finally {
      setBusy(submitBtn, false);
    }
  });
}

function setupCreateTopologyForm() {
  const form = document.getElementById('createTopology');
  const status = document.getElementById('createTopologyStatus');
  if (!form || !status) return;

  const submitBtn = form.querySelector('button[type="submit"]');

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    setBusy(submitBtn, true);
    setStatus(status, 'Creating topology…');

    const fd = new FormData(form);
    const name = String(fd.get('name') || '').trim();
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

    try {
      const res = await createTopology({ name, levels, assignments });
      if (res.errors && res.errors.length) {
        setStatus(status, `Created ${res.topologyName}. Patched ${res.patchedNodes}. Errors: ${res.errors.length}`, res.errors);
      } else {
        setStatus(status, `Created ${res.topologyName}. Patched ${res.patchedNodes} node(s).`);
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
  container.innerHTML = '';

  const domain = domainById(viz.topology, state.selectedDomainId) || viz.topology;
  const allowed = new Set(domain.nodeNames);

  const blocksByNode = new Map();
  for (const b of viz.blocks) {
    if (!allowed.has(b.nodeName)) continue;
    if (!blocksByNode.has(b.nodeName)) blocksByNode.set(b.nodeName, []);
    blocksByNode.get(b.nodeName).push(b);
  }

  const nodes = viz.nodes.filter(n => allowed.has(n.name));
  for (const node of nodes) {
    const nodeBlocks = (blocksByNode.get(node.name) || []).slice();
    // Group blocks by GPU index and stack.
    const byGpu = Array.from({ length: node.gpuCount }, () => []);
    for (const b of nodeBlocks) {
      const idx = Math.max(0, Math.min(node.gpuCount - 1, b.gpuIndex));
      byGpu[idx].push(b);
    }

    const board = el('div', { class: 'board' });
    for (let gi = 0; gi < node.gpuCount; gi++) {
      const col = el('div', { class: 'col', title: `GPU ${gi}` });

      let offset = 0;
      for (const b of byGpu[gi]) {
        const h = Math.max(0.05, Math.min(1, b.height));
        const px = Math.round(h * 180);
        const block = el('div', {
          class: 'block',
          style: `bottom:${offset}px;height:${px}px;background:${hashColor(b.colorKey)};`,
          title: `${b.namespace}/${b.pod} (gpu ${gi}, ${b.height.toFixed(2)} gpu)`
        });
        block.appendChild(el('div', { class: 'block-label' }, [b.pod]));
        col.appendChild(block);
        offset += px;
        if (offset >= 180) break;
      }

      board.appendChild(col);
    }

    const nodeEl = el('div', { class: 'node' }, [
      el('div', { class: 'node-title' }, [`${node.name} — ${node.gpuCount} GPU(s)`]),
      board
    ]);

    container.appendChild(nodeEl);
  }

  if (nodes.length === 0) {
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
    renderPending(viz);
    renderNodes(viz);
  } catch (e) {
    document.getElementById('meta').textContent = `Error: ${e.message}`;
  }
}

refresh();
setupCreatePodForm();
setupCreateTopologyForm();
setInterval(refresh, 5000);
