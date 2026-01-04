# Scheduler Configuration Customization

## Problem

The operator generates a hardcoded scheduler ConfigMap. Users cannot:
- Add or remove actions and plugins
- Modify plugin arguments (only a few are exposed via dedicated fields)

---

## Option 1: Unified Plugin Configuration (Recommended)

Add structured fields to `SchedulingShardSpec` for plugin and action configuration.

```yaml
apiVersion: kai.scheduler/v1
kind: SchedulingShard
metadata:
  name: my-shard
spec:
  # Basic fields (unchanged)
  partitionLabelValue: "pool-a"
  placementStrategy:
    gpu: binpack
    cpu: binpack
  kValue: 1.5
  minRuntime:
    preemptMinRuntime: "5m"
    reclaimMinRuntime: "10m"

  # NEW: Built-in plugins (merged with defaults)
  plugins:
    minruntime:
      enabled: false # Disable this plugin
    proportion:
      arguments: # Override arguments
        kValue: "1.5"
    nodeplacement:
      priority: 500 # Change ordering
      arguments:
        gpu: spread

  # NEW: Custom plugins
  additionalPlugins:
    mycustomplugin:
      priority: 250
      arguments:
        key: value

  # NEW: Actions configuration
  actions:
    consolidation:
      enabled: false
      priority: 100
  
  additionalActions:
    mycustomaction:
      enabled: true
      priority: 50
```

**Plugin/Action priorities** (for ordering):

Each action and plugin has a configurable priority for ordering.
The default priorities determine the default order.

**Merge behavior**: Unmentioned plugins/actions use defaults. New plugins from KAI upgrades are auto-included.

**Pros**: Explicit state, upgrade-safe

**Cons**: More complex API surface, makes our current "internal(?)" scheduler configuration public API that we must maintain.

---

## Option 2: External ConfigMap Reference

Reference a user-managed ConfigMap.

```yaml
spec:
  schedulerConfiguration:
    configMapRef:
      name: my-scheduler-config
    mergeStrategy: overlay # or: replace
```

MergeStrategy `overlay` will use strategic merge patch to merge the user's config with the default config.

**Pros**: Separation of concerns, GitOps-friendly

**Cons**: Two sources of truth, sync issues, harder to validate, limited ordering control, no type safety, user needs to figure out the correct config format, 

---

## Handling Existing Fields

### Fields That Affect Plugins/Actions

| Existing Field | Effect |
|----------------|--------|
| `kValue` | `proportion` plugin arguments |
| `minRuntime` | `minruntime` plugin arguments |
| `placementStrategy.gpu` | `gpupack` vs `gpuspread`, `nodeplacement` args |
| `placementStrategy.cpu` | `nodeplacement` args |
| `placementStrategy` (any spread) | Disables `consolidation` action |

### Recommended Approach: Internal Transformation

1. Existing fields (`placementStrategy`, `kValue`, etc.) are converted to plugin config internally
2. User's `plugins`/`actions` are merged on top (take precedence)

```yaml
spec:
  placementStrategy:
    gpu: spread # Converted internally
  plugins:
    nodeplacement:
      arguments:
        cpu: binpack # User override on top
```

We might want to deprecate the old fields in a backwards compatible way.

---

## Questions for Discussion

1. What specific customizations are users requesting?
2. Should the new configuration fields be added under `advanced` section or top level?
3. Should we deprecate `minRuntime`, `kValue` and `placementStrategy` fields?
