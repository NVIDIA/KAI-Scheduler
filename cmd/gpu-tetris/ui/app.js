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

function setupCreatePodForm() {
  const form = document.getElementById('createPod');
  const status = document.getElementById('createPodStatus');
  if (!form || !status) return;

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    status.textContent = 'Creating…';

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
      status.textContent = `Created ${res.namespace}/${res.name}. Waiting for scheduling…`;
      await refresh();
    } catch (err) {
      status.textContent = `Error: ${err.message}`;
    }
  });
}

function renderTopology(viz) {
  const root = document.getElementById('topology');
  root.innerHTML = '';

  function renderNode(n) {
    const btn = el('button', {
      class: `tree-button ${state.selectedDomainId === n.id ? 'active' : ''}`,
      onclick: () => {
        state.selectedDomainId = n.id;
        renderTopology(viz);
        renderNodes(viz);
      },
      title: `${n.nodeNames.length} node(s)`
    }, [`${n.name} (${n.nodeNames.length})`]);

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
    renderNodes(viz);
  } catch (e) {
    document.getElementById('meta').textContent = `Error: ${e.message}`;
  }
}

refresh();
setupCreatePodForm();
setInterval(refresh, 5000);
