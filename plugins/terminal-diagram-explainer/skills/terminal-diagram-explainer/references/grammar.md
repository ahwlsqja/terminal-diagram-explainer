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
%% comment
```

Limits: 256 KiB input, 2,048 lines, 48 nodes, 96 edges, 64-byte ID, 96-cell label, 240×200 output.

Unsupported syntax fails rather than falling back: cycle, subgraph, class/style/click, HTML/Markdown labels, sequence/ER, `RL`, `BT`.
