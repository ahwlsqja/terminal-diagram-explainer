# Renderer grammar

```text
flowchart LR|TD|TB
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
%% comment
```

Limits: 256 KiB input, 2,048 lines, 48 nodes, 96 edges, 64-byte ID, 96-cell label, 240×200 output.

Cycle and self-loop edges render through an outer gutter. Their labels appear once in a bounded `feedback:` legend.

Forward edges that skip an intermediate rank also use an outer gutter. Labeled routes appear once in a bounded `routed:` legend.

Unsupported syntax fails rather than falling back: subgraph, class/style/click, HTML/Markdown labels, sequence/ER, `RL`, `BT`.
