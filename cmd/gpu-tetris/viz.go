package main

import (
	"fmt"
	"hash/fnv"
	"math"
	"regexp"
	"sort"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"

	kaiv1alpha1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1alpha1"
	schedulingv1alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v1alpha2"
	enginev2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	snapshotplugin "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/snapshot"
)

type Viz struct {
	GeneratedAt string          `json:"generatedAt"`
	Topology    DomainNode      `json:"topology"`
	Nodes       []NodeViz       `json:"nodes"`
	Blocks      []BlockViz      `json:"blocks"`
	Pending     []PendingPodViz `json:"pending"`
	Queues      []QueueViz      `json:"queues"`
}

type QueueViz struct {
	Name         string     `json:"name"`
	DisplayName  string     `json:"displayName"`
	ParentQueue  string     `json:"parentQueue"`
	Children     []QueueViz `json:"children"`
	AllocatedGPU float64    `json:"allocatedGpu"`
	RequestedGPU float64    `json:"requestedGpu"`
	Priority     int        `json:"priority"`
}

type PendingPodViz struct {
	ID        string `json:"id"`
	Pod       string `json:"pod"`
	Namespace string `json:"namespace"`
	Queue     string `json:"queue"`
	Request   string `json:"request"`
	CreatedAt string `json:"createdAt"`
	Reason    string `json:"reason"`
}

type DomainNode struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	NodeNames []string     `json:"nodeNames"`
	Children  []DomainNode `json:"children"`
}

type NodeViz struct {
	Name     string `json:"name"`
	GPUCount int    `json:"gpuCount"`
}

type BlockViz struct {
	ID        string  `json:"id"`
	NodeName  string  `json:"nodeName"`
	GPUIndex  int     `json:"gpuIndex"`
	Pod       string  `json:"pod"`
	Namespace string  `json:"namespace"`
	Height    float64 `json:"height"`   // 0..1 (fraction of a GPU)
	ColorKey  string  `json:"colorKey"` // used by UI to derive color
}

func BuildViz(snap *snapshotplugin.Snapshot) (*Viz, error) {
	nodes := make([]NodeViz, 0)
	nodeGPUCounts := map[string]int{}
	if snap == nil || snap.RawObjects == nil {
		return nil, fmt.Errorf("snapshot missing rawObjects")
	}

	for _, n := range snap.RawObjects.Nodes {
		if n == nil {
			continue
		}
		gpuCount := getNodeGPUCount(n)
		if gpuCount <= 0 {
			continue
		}
		nodes = append(nodes, NodeViz{Name: n.Name, GPUCount: gpuCount})
		nodeGPUCounts[n.Name] = gpuCount
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })

	topoRoot := buildTopologyRoot(snap.RawObjects.Topologies, snap.RawObjects.Nodes, nodeGPUCounts)
	blocks := buildBlocks(snap.RawObjects.BindRequests, snap.RawObjects.Pods, nodeGPUCounts)
	pending := buildPendingPods(snap.RawObjects.Pods)
	queues := buildQueues(snap.RawObjects.Queues)

	return &Viz{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Topology:    topoRoot,
		Nodes:       nodes,
		Blocks:      blocks,
		Pending:     pending,
		Queues:      queues,
	}, nil
}

func buildPendingPods(pods []*v1.Pod) []PendingPodViz {
	pending := make([]PendingPodViz, 0)
	for _, p := range pods {
		if p == nil {
			continue
		}
		if p.Spec.NodeName != "" {
			continue
		}
		if p.Status.Phase != v1.PodPending {
			continue
		}
		req := podGPURequestSummary(p)
		if req == "" {
			continue
		}
		queue := ""
		if p.Labels != nil {
			queue = p.Labels["kai.scheduler/queue"]
		}
		reason := podPendingReason(p)
		createdAt := ""
		if !p.CreationTimestamp.IsZero() {
			createdAt = p.CreationTimestamp.Time.UTC().Format(time.RFC3339)
		}
		pending = append(pending, PendingPodViz{
			ID:        fmt.Sprintf("%s/%s", p.Namespace, p.Name),
			Pod:       p.Name,
			Namespace: p.Namespace,
			Queue:     queue,
			Request:   req,
			CreatedAt: createdAt,
			Reason:    reason,
		})
	}
	sort.Slice(pending, func(i, j int) bool {
		if pending[i].CreatedAt == "" {
			return false
		}
		if pending[j].CreatedAt == "" {
			return true
		}
		return pending[i].CreatedAt < pending[j].CreatedAt
	})
	return pending
}

func podGPURequestSummary(p *v1.Pod) string {
	whole := int64(0)
	for _, c := range p.Spec.Containers {
		if q, ok := c.Resources.Requests[v1.ResourceName("nvidia.com/gpu")]; ok {
			whole += q.Value()
			continue
		}
		if q, ok := c.Resources.Limits[v1.ResourceName("nvidia.com/gpu")]; ok {
			whole += q.Value()
		}
	}
	if whole > 0 {
		return fmt.Sprintf("%dgpu", whole)
	}

	ann := p.Annotations
	if ann == nil {
		return ""
	}
	if fracStr, ok := ann["gpu-fraction"]; ok {
		frac, err := strconv.ParseFloat(fracStr, 64)
		if err == nil && frac > 0 {
			if devStr, ok := ann["gpu-fraction-num-devices"]; ok {
				if dev, err := strconv.Atoi(devStr); err == nil && dev > 1 {
					return fmt.Sprintf("%.2fgpu × %d", frac, dev)
				}
			}
			return fmt.Sprintf("%.2fgpu", frac)
		}
	}
	if memStr, ok := ann["gpu-memory"]; ok {
		if mem, err := strconv.Atoi(memStr); err == nil && mem > 0 {
			return fmt.Sprintf("%dMiB", mem)
		}
	}
	return ""
}

func podPendingReason(p *v1.Pod) string {
	for _, c := range p.Status.Conditions {
		if c.Type != v1.PodScheduled {
			continue
		}
		if c.Status == v1.ConditionFalse {
			if c.Reason != "" {
				return c.Reason
			}
			return c.Message
		}
	}
	return ""
}

func getNodeGPUCount(n *v1.Node) int {
	q, found := n.Status.Allocatable[v1.ResourceName("nvidia.com/gpu")]
	if !found {
		return 0
	}
	v := q.Value()
	if v <= 0 {
		return 0
	}
	if v > math.MaxInt32 {
		return 0
	}
	return int(v)
}

func buildBlocks(bindRequests []*schedulingv1alpha2.BindRequest, pods []*v1.Pod, nodeGPUCounts map[string]int) []BlockViz {
	blocks := make([]BlockViz, 0)
	if len(bindRequests) == 0 {
		return blocks
	}

	// Build pod-to-queue lookup map
	podQueues := make(map[string]string) // key: "namespace/podName" -> queue name
	for _, p := range pods {
		if p == nil || p.Labels == nil {
			continue
		}
		if queue := p.Labels["kai.scheduler/queue"]; queue != "" {
			podQueues[p.Namespace+"/"+p.Name] = queue
		}
	}

	for _, br := range bindRequests {
		if br == nil {
			continue
		}
		if br.Status.Phase != schedulingv1alpha2.BindRequestPhaseSucceeded {
			continue
		}
		nodeName := br.Spec.SelectedNode
		if nodeName == "" {
			continue
		}
		gpuCount := nodeGPUCounts[nodeName]
		if gpuCount <= 0 {
			continue
		}
		if br.Spec.ReceivedGPU == nil || br.Spec.ReceivedGPU.Count <= 0 {
			continue
		}

		portion := parsePortion(br.Spec.ReceivedGPU.Portion)
		if portion <= 0 {
			portion = 1
		}
		if portion > 1 {
			portion = 1
		}

		podName := br.Spec.PodName
		ns := br.Namespace
		if ns == "" {
			ns = "default"
		}

		// Use queue as colorKey, fallback to namespace if not found
		colorKey := podQueues[ns+"/"+podName]
		if colorKey == "" {
			colorKey = ns
		}

		gpuIndexes := gpuIndexesFromGroups(br.Spec.SelectedGPUGroups, br.Spec.ReceivedGPU.Count, gpuCount, podName)
		for i, idx := range gpuIndexes {
			blocks = append(blocks, BlockViz{
				ID:        stableID(ns, podName, nodeName, i),
				NodeName:  nodeName,
				GPUIndex:  idx,
				Pod:       podName,
				Namespace: ns,
				Height:    portion,
				ColorKey:  colorKey,
			})
		}
	}

	// Stable sort for nicer display.
	sort.Slice(blocks, func(i, j int) bool {
		if blocks[i].NodeName != blocks[j].NodeName {
			return blocks[i].NodeName < blocks[j].NodeName
		}
		if blocks[i].GPUIndex != blocks[j].GPUIndex {
			return blocks[i].GPUIndex < blocks[j].GPUIndex
		}
		if blocks[i].Namespace != blocks[j].Namespace {
			return blocks[i].Namespace < blocks[j].Namespace
		}
		return blocks[i].Pod < blocks[j].Pod
	})

	return blocks
}

func parsePortion(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

var firstIntRe = regexp.MustCompile(`\d+`)

func gpuIndexesFromGroups(groups []string, count int, gpuCount int, seed string) []int {
	idxs := make([]int, 0, count)
	for _, g := range groups {
		m := firstIntRe.FindString(g)
		if m == "" {
			continue
		}
		v, err := strconv.Atoi(m)
		if err != nil {
			continue
		}
		if gpuCount > 0 {
			v = ((v % gpuCount) + gpuCount) % gpuCount
		}
		idxs = append(idxs, v)
		if len(idxs) >= count {
			break
		}
	}
	for len(idxs) < count {
		idxs = append(idxs, int(hashToRange(seed+fmt.Sprintf("-%d", len(idxs)), gpuCount)))
	}
	return idxs
}

func stableID(ns, pod, node string, part int) string {
	return fmt.Sprintf("%s/%s@%s#%d", ns, pod, node, part)
}

func hashToRange(s string, max int) uint32 {
	if max <= 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32() % uint32(max)
}

func buildTopologyRoot(topologies []*kaiv1alpha1.Topology, allNodes []*v1.Node, nodeGPUCounts map[string]int) DomainNode {
	// Filter only nodes that have GPUs (since this is a GPU viz).
	gpuNodes := make([]*v1.Node, 0, len(allNodes))
	for _, n := range allNodes {
		if n == nil {
			continue
		}
		if nodeGPUCounts[n.Name] > 0 {
			gpuNodes = append(gpuNodes, n)
		}
	}

	if len(gpuNodes) == 0 {
		return DomainNode{ID: "root", Name: "cluster", NodeNames: []string{}}
	}

	var topo *kaiv1alpha1.Topology
	if len(topologies) > 0 {
		topo = topologies[0]
	}
	if topo == nil || len(topo.Spec.Levels) == 0 {
		names := make([]string, 0, len(gpuNodes))
		for _, n := range gpuNodes {
			names = append(names, n.Name)
		}
		sort.Strings(names)
		return DomainNode{ID: "root", Name: "cluster", NodeNames: names}
	}

	root := &domainBuilder{node: DomainNode{ID: "root", Name: topo.Name}}
	for _, n := range gpuNodes {
		root.addNode(topo.Spec.Levels, n)
	}
	root.finalize()
	return root.node
}

type domainBuilder struct {
	node     DomainNode
	children map[string]*domainBuilder
}

func (d *domainBuilder) addNode(levels []kaiv1alpha1.TopologyLevel, n *v1.Node) {
	cur := d
	cur.node.NodeNames = append(cur.node.NodeNames, n.Name)

	for _, lvl := range levels {
		key := lvl.NodeLabel
		val := "(none)"
		if n.Labels != nil {
			if v, ok := n.Labels[key]; ok && v != "" {
				val = v
			}
		}
		id := cur.node.ID + "/" + key + "=" + val
		name := val
		if cur.children == nil {
			cur.children = map[string]*domainBuilder{}
		}
		nxt, ok := cur.children[id]
		if !ok {
			nxt = &domainBuilder{node: DomainNode{ID: id, Name: name}}
			cur.children[id] = nxt
		}
		cur = nxt
		cur.node.NodeNames = append(cur.node.NodeNames, n.Name)
	}
}

func (d *domainBuilder) finalize() {
	sort.Strings(d.node.NodeNames)
	if len(d.node.NodeNames) > 1 {
		// de-dupe
		uniq := d.node.NodeNames[:0]
		var prev string
		for _, v := range d.node.NodeNames {
			if v == prev {
				continue
			}
			uniq = append(uniq, v)
			prev = v
		}
		d.node.NodeNames = uniq
	}

	if len(d.children) == 0 {
		return
	}

	ids := make([]string, 0, len(d.children))
	for id := range d.children {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		child := d.children[id]
		child.finalize()
		d.node.Children = append(d.node.Children, child.node)
	}
}

func buildQueues(queues []*enginev2.Queue) []QueueViz {
	if len(queues) == 0 {
		return []QueueViz{}
	}

	// Build a map of queue name -> queue builder for easy lookup
	type queueBuilder struct {
		viz      QueueViz
		children []*queueBuilder
	}
	queueMap := make(map[string]*queueBuilder)

	for _, q := range queues {
		if q == nil {
			continue
		}
		priority := 100 // default priority
		if q.Spec.Priority != nil {
			priority = *q.Spec.Priority
		}
		displayName := q.Spec.DisplayName
		if displayName == "" {
			displayName = q.Name
		}

		qb := &queueBuilder{
			viz: QueueViz{
				Name:         q.Name,
				DisplayName:  displayName,
				ParentQueue:  q.Spec.ParentQueue,
				Children:     []QueueViz{},
				AllocatedGPU: extractGPUQuantity(q.Status.Allocated),
				RequestedGPU: extractGPUQuantity(q.Status.Requested),
				Priority:     priority,
			},
			children: []*queueBuilder{},
		}
		queueMap[q.Name] = qb
	}

	// Build parent-child relationships
	roots := make([]*queueBuilder, 0)
	for _, qb := range queueMap {
		if qb.viz.ParentQueue == "" {
			roots = append(roots, qb)
		} else if parent, ok := queueMap[qb.viz.ParentQueue]; ok {
			parent.children = append(parent.children, qb)
		} else {
			// Parent not found, treat as root
			roots = append(roots, qb)
		}
	}

	// Recursively build QueueViz tree from builders
	var buildVizTree func(qb *queueBuilder) QueueViz
	buildVizTree = func(qb *queueBuilder) QueueViz {
		result := qb.viz
		// Sort children by name
		sort.Slice(qb.children, func(i, j int) bool {
			return qb.children[i].viz.Name < qb.children[j].viz.Name
		})
		for _, child := range qb.children {
			result.Children = append(result.Children, buildVizTree(child))
		}
		return result
	}

	// Sort roots by name
	sort.Slice(roots, func(i, j int) bool { return roots[i].viz.Name < roots[j].viz.Name })

	result := make([]QueueViz, 0, len(roots))
	for _, r := range roots {
		result = append(result, buildVizTree(r))
	}

	return result
}

func extractGPUQuantity(resources v1.ResourceList) float64 {
	if resources == nil {
		return 0
	}
	gpuQuantity, ok := resources[v1.ResourceName("nvidia.com/gpu")]
	if !ok {
		return 0
	}
	// GPU quantities can be fractional, so use AsApproximateFloat64
	return gpuQuantity.AsApproximateFloat64()
}
