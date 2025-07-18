# Time Aware Fairness
<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [User Stories (Optional)](#user-stories-optional)
    - [Story 1](#story-1-proportional-time-sharing-of-resoucres)
    - [Story 2](#story-2-burst-access-to-resources)
    - [Story 3](#story-3-time-aware-fairness-with-guaranteed-resources)
    - [Story 4](#story-4-no-over-usage-open-for-debate)
  - [Notes/Constraints/Caveats (Optional)](#notesconstraintscaveats)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Design Details](#design-details)
  - [Vacant-adjusted normalized usage](#vacant-adjusted-normalized-usage)
<!-- /toc -->

## Summary

This document aims to describe how the scheduler can make use of historical resource usage by queues and users in order to achieve fairness of resource allocation over time. The mechanism of storing and serving the necessary data is out of scope for this document and will be described in a separate design doc.  
The scheduler will consider past resource allocations when dividing over-quota resources to queues in the DRF algorithm: queues that have used more resource-hours should be penalized compared to queues that have not used resources recently. This will effect the over-quota resource division only, meaning that deserved quota and queue priority will take precedence.  
This will be an opt-in feature: if the admin does not configure time-aware-fairness, the scheduler will behave exactly the same as before.

## Motivation

Time-aware fairness is a desirable behavior for many use cases, as evident by the popularity of HPC systems like slurm, and by the demand for slurm-like schedulers in k8s. Implementing time-aware fairness in KAI will offer users a k8s native scheduler that can offer deserved quotas, for inference and interactive use cases, alongside an HPC-like experience for batch job queueing. See the [user stories](#user-stories) section below for detailed use cases.

### Goals

* Allow fair time-sharing of resources: prioritize queues that have not accessed resources recently over queues that did. Other than deserved quotas, resources should be fairly allocated over time: queues with identical weights and requests will receive similar allocation of resource-hours over a long enough period of time.
* Allow burst access to resources: allow queues in busy clusters to access more resources than their weight for short periods of time.
* Fallback to current behavior: if over-time feature is not configured, the scheduler will behave exactly like today, with classic DRF determining fair share.
* Don't break current guarantees of deserved resources and over-quota priority. 

### Non-Goals
* Guarantee feature parity with any existing HPC system
* In-depth design of the accounting service: this will be addressed in a separate document

## Proposal

Track usage data of resources by queues over time, with a configurable decay formula and reset period. Use this usage data in DRF calculations when allocating resources: per resource, 
<!--
This is where we get down to the specifics of what the proposal actually is.
This should have enough detail that reviewers can understand exactly what
you're proposing, but should not include things like API designs or
implementation. What is the desired outcome and how do we measure success?.
The "Design Details" section below is for the real
nitty-gritty.
-->

### User Stories

<!--
Detail the things that people will be able to do if this KEP is implemented.
Include as much detail as possible so that people can understand the "how" of
the system. The goal here is to make this feel real for users without getting
bogged down.
-->

#### Story 1: Proportional time sharing of resources
Assume an admin manages a cluster with X GPUs. Two users share the cluster, each wants to run an infinite series batch jobs, each requiring X GPUs allocated for an hour. The admin assigns no deserved quota and identical over-quota weight to each users' queue.  
Under non-time-aware DRF, the fair share of GPUs for each queue will be X/2. No reclaims will occur, and the allocated job will revert to arbitrary tie-breakers in queue order, such as queue creation timestamp, resulting in one of the queues always getting access to resources, assuming an infinite backlog of jobs:

![fig 1: job allocation with strict DRF](strict-drf-fig-1.svg)

However, a time-aware scheduling algorithm will decrease the fair share for the queue of the running job, and increase it for the starved queue, resulting in an oscillating allocation of jobs between them:

![fig 2: job allocation with time aware fairness](time-aware-fairness-fig-2.svg)

This, potentially coupled with min-runtime protections for added guarantees, can ensure a non-disruptive oscillation between the queues' access to resources.

#### Story 2: Burst access to resources
In a busy cluster with several queues used mainly for training job, one unprioritized queue "`Q`" has a quota of 1/10th of the cluster's resources. `Q` needs to run a job requiring 2/10th of the resources for a relatively short period. Assuming a busy cluster, using normal DRF, it's unlikely that `Q` will ever get access to enough resources in order to run the job, effectively starving it. However, using time-aware fairness, the rest of the queues' fair share will decrease as they're using resources over time, eventually resulting in `Q` having enough fair share to allocate it's job for long enough to run it. A short min-runtime period for `Q` can allow it to run it's job to completion once it gets access to the resources.

#### Story 3: Time aware fairness with guaranteed resources
In a heterogeneous cluster combining interactive, critical inference, and training use cases, different users with different queues require reliable access to deserved resources, while time-sharing over-quota resources for training jobs. Admin assigns deserved quota for each queue based on the users' needs, allowing them to run inference and some interactive workloads for use cases such as data exploration and small-scale experiments with jupyter notebooks and similar tools.  
Users can rely on guaranteed resources for non-preemptible interactive & inference workloads, while queueing train jobs. Time-aware fairness will ensure fair access to resources, roughly proportional to the weight assigned to the different queues. Interactive and inference workloads will not be interrupted due to the usage of in-quota resources & non-preemptible properties.

#### Story 4: No over-usage (open for debate)
In a cluster with no deserved quotas, N queues are weighted arbitrarily (W_i for i in N is queue `i`'s weight, Ẃ_i = W_i/∑W_j for i,j in N is the normalized weight). If no queue ever goes above it's relative share - should they be penalized for their usage? On one hand, it's not intuitively fair to be penalized for usage of one's relative share. On the other, this could cause any queue with a pending job that requires more than it's relative share to be starved. For example, SDRF doesn't count in-share usage as "Commitment".

### Notes/Constraints/Caveats

#### Data storage constraints
Regardless of which backend TSDB is chosen for the storage of resource usage data, it's to be expected that busy, large-scale clusters will generate a lot of it, and there could be limitations to the granularity of data preserved, and thus the accuracy of results achieved. It's assumed that some loss of granularity might be required in order to save data for long periods of time: admins might need to balance the historical time window with the data granularity and the storage & compute resources required from the backend data storage; KAI-scheduler needs to allow the admins to configure those to their liking.

#### Intuitive understanding of fairness over time
Several parameters and configurations can effect the outcome of the scheduling algorithm: good documentation, and potentially tools, will need to be provided in order to help the admin configure the cluster properly. An intuitive configuration should result in behavior that is predictable in simple scenarios: for example, "Using X GPUs for T hours will result in a project with W over-quota weight getting penalized by a factor of 50%". Several variables can go into achieving this result - we need to map these in a clear way.

### Risks and Mitigations

<!--
ToDo
-->

## Design Details

> **⚠️ Work in Progress**  
> This design is currently under development. The mathematical formulation and implementation details are an initial proposal for further discussion. Feedback and suggestions are welcome.

Accounting service will provide the scheduler with data for each cycle about the usage of resources per queue per resource. A vector will represent the usage of each queue per resource type.  
The resource division formula for over-quota resource can be found in proportion plugin, resource_division.go:
```Go
fairShare := amountToGiveInCurrentRound * (overQuotaWeight / totalWeights)
```
This calculation is done for each resource type. The result of this calculation is later capped with the requested resource of the queue, so each round of this calculation for all queues could result in remaining capcacity to divide - this is run in rounds, each time with the updated remaining unsatisfied queues, untill all resources are divided or all queues get their request.

For convenience, we'll write it out as a formula:

$$\text{F} = C \cdot \frac{W}{\sum{W}}$$

Where:
- **F** is the fair share allocated to a specific queue in the current round
- **C** is the remaining capacity (max amount to give in current round)
- **w** is the weight for a specific queue
- **∑w** is the sum of weights across all competing queues

Let's also mark the normalized weight as:

$$W'_i = \frac{W_i}{\sum{W}}$$


### Vacant-adjusted normalized usage

First, we'll define a normalized "usage" score for each queue and resource:

$$U'_i = \frac{U_i}{\sum{U}}$$

We can furthermore consider the **unallocated** resources in the cluster. If we go back to the definition of $U'_i$, we can add:

$$U'_i = \frac{U_i}{\sum{U} + V}$$

where V (vacant) represents the **unallocated** resources for the considered time period. This will add the benefit of reduced penalty for usages that were relatively small compared to free resources in the cluster - otherwise, for example, a usage of a single GPU*second in an empty cluster will severely punishing queues that utilized small amounts of clusters when there was no contention.

Now, we can plug in $U'_i$ to our fair share calculation:

$$F_i = C \cdot (W'_i - U'_i)$$

Which will give us the following:
* When usage is zero for all queues, we revert back to the current calculation
* Decreased fair share for queues that are using relatively more resources, while still giving more "breathing room" for queues with larger weights
* Queues that their relative **usage** is bigger than their relative **share** will get a negative value. This needs to be addressed somehow, maybe flooring it with 0 or normalizing the $(W'_i-U'_i)$ values - needs further thought

If there are still unclaimed resources, the relative weights and usage values will be re-calculated in the next round - allowing even very over-using queues to get access to resources, if they are left unclaimed by all other queues.

To simplify this concept, we will define $U'_i$ as the relative usage to the entire cluster's capacity for the relevant time period:

$$U'_i = \frac{U_i}{\sum{U} + V} = \frac{U_i}{T} $$

Where T is the total amount of resources in the cluster for the relevant period. In a busy cluster with no free resources, $\sum{U}=T$

#### Considering vacant resources:
There are a few potential upsides for considering vacant resources when calculating normalized usages. First, we can demonstrate the value with an example:

First, it's easy to see that in a buys cluster, as $V$ approaches 0 relative to $\sum{U}$, the normalized usage approaches what it would be if we didn't consider $V$ at all: in other words, in busy clusters, it doesn't really matter if we vacancy-adjust our usages or not.

Now, let's consider what happens when the cluster is relatively empty:

Consider a cluster with 100 GPUs. We have one queue, $Q_1$, with weight of 50. We also have 50 smaller queues with weight of 1 each.
This gives us a normalized weight for $Q_1$ of 0.5.

Now, let's say that the cluster is completely empty, and $Q_1$ used a small amount of resources: say, 0.1 GPU-hours. In a cluster of 100 GPUs, this is obviously not a significant amount.

If we don't vacant-adjust our usage, the normalized usage of Q1 will be $U'_1 = 1$, which will severely punish it: in fact, the the previously proposed equasion, it will result in a negative value:

$$F_1 = C \cdot (W'_1 - U'_1) = C \cdot (0.5 - 1) = -0.5 \cdot C$$

*Wether we floor the value with 0 or normalize the result -* $F_1$ *will still be 0*

$Q_1$, the most prioritized queue in the cluster by far, will be severely punished for a minute amount of resources allocated from an empty cluster.

On the other hand - if we consider vacant resources, we get the following:

$$U'_1 = \frac{U_1}{\sum{U} + V} = \frac{U_1}{T} = \frac{0.1}{100}$$

*Assuming that we're looking at a time period of 1h, and that the units for usage are GPU-hours*

The value of $U'_1$ approaches 0, reflecting it's actual usage of cluster resources. The 0.1 GPU-hours will not result in significant penalty. 

However, if it used 50 GPUs for that duration, it will be 0.5. Plugged to the resource division equation, we will get:

$$U'_1 = \frac{U_1}{T} = \frac{50}{100} = 0.5$$

$$F_1 = C \cdot (W'_1 - U'_1) = C \cdot (0.5 - 0.5) = 0 \cdot C$$

Which means that it will get no fair share if it used half the cluster resources for the relevant period.

#### Normalizing usage

Let's take another look at our resource division equation:

$$F_i = C \cdot (W'_i - U'_i)$$

Since $W'_i$ is normalized, $\sum{W} = 1$. This is convenient since it means that we can potentially divide the entire cluster capacity, marked here as $C$, in one round. The result of this calculation is later capped by the queue request, so the calculation repeats until no resources are left, or until all queues get their requested resources.

However, now, when $U'_i$ will start receiving positive values, we will get cases where:

$$\sum{F} = C \cdot\sum{(W'-U')} < C$$

One way to address this is to run more rounds, but it will be easier to just normalize the factor.

Another issue to consider is that we can't divide negative resources - therefore, we need to apply $\max{\{(W'-U'), 0\}}$ before normalizing.

Let's mark $P$ for "portion" and define:

$$P_i = \max{\{W'_i - U'_i, 0\}}$$

$$P'_i = \frac{P_i}{\sum{P}}$$

*Where* $\sum{P}$ *considers only queues that participate in the current resource division round*

Which gives us:

$$\sum{P'} = 1$$

$$0<=P'_i<=1$$

For each resource division round, which makes it simpler to divide resources.