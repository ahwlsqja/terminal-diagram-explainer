# Renderer grammar

```text
flowchart LR|TD|TB
subgraph ScopeID[scope label]
NodeID
NodeID[process label]
NodeID{decision label}
NodeID[(data store or VIEW)]
A --> B
A -->|success| B
A -.->|failure or async| C
A --> B --> C
A --> B
B -.->|retry| A
end
%% comment
```

Limits: 256 KiB input, 2,048 lines, 48 nodes, 96 edges, 32 subgraphs, subgraph depth 8, 64-byte ID, 96-cell label, 240×200 output.

Subgraphs form a bounded parent forest. Node and subgraph IDs share one graph-wide namespace, and every node has exactly one direct scope. Existing bare edge endpoints may be referenced across scopes without changing ownership. Empty leaf subgraphs fail closed.

Cycle and self-loop edges render through an outer gutter. Their labels appear once in a bounded `feedback:` legend.

Forward edges that skip an intermediate rank also use an outer gutter. Labeled routes appear once in a bounded `routed:` legend.

Cross-subgraph edges use frame-safe outer corridors and place labels in the bounded `routed:` legend.

Unsupported syntax fails rather than falling back: class/style/click, HTML/Markdown labels, sequence/ER, `RL`, `BT`.
