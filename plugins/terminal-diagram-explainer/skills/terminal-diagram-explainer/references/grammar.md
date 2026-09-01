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

```text
sequenceDiagram
participant Client as Browser Client
participant API as API Gateway
Client ->> API: request
API -->> Client: response
API ->> API: record metrics
```

Sequence limits: 16 participants, 96 messages, 64-byte ID, 96-cell label. Declare all participants before messages. Endpoint references use IDs, `->>` is request, `-->>` is return, and the first `:` starts the required rest-of-line label. Repeated messages from one sender express fan-out; same endpoints express a self-message.

```text
loop retry up to 3
  Client ->> API: request
end
alt accepted
  API -->> Client: 202
else rejected
  API -->> Client: 400
end
opt audit
  Client ->> Client: metrics
end
```

Structured fragment limits: 32 fragments, nesting depth 8, 256 total message/control steps. `loop` and `opt` have one nonempty branch. `alt` requires exactly one `else` and both branches must contain a message.

```text
activate API
Client ->> API: request
API -->> Client: response
deactivate API
```

Explicit activation limits: 96 starts, per-participant LIFO depth 8. A pair must contain a message and cannot cross a fragment open, branch, or end boundary. It is a visual serialized-timeline interval, not proof of a runtime call stack.

```text
par email
  API ->> Email: send
and sms
  API ->> SMS: send
end
```

`par` requires at least two nonempty branches. The renderer labels it `par (display order only)`: branch vertical/source order is presentation order, not simultaneous execution order or a happens-before relation. Activation pairs must close inside one branch.

```text
erDiagram
Customer ||--o{ Order : places
Customer[고객] {
  uuid tenant_id
  uuid id
  string email UNIQUE NOT NULL
  PRIMARY KEY (tenant_id, id)
}
Order[주문] {
  uuid tenant_id
  uuid id
  uuid customer_id
  PRIMARY KEY (tenant_id, id)
  FOREIGN KEY (tenant_id, customer_id) REFERENCES Customer(tenant_id, id)
}
```

ER limits: 32 entities, 64 relationships, 192 attributes total and 32 per entity. Entity IDs and display labels are unique. Attributes use `type name [PK] [FK] [UNIQUE] [NOT NULL]`; marker input order is free and output canonicalizes to `PK FK UNIQUE NOT NULL type name`. Multiline blocks also support 2–8-column `PRIMARY KEY (...)`, `UNIQUE (...)`, and `FOREIGN KEY (...) REFERENCES Entity(...)`; these three leading keywords are reserved inside entity bodies. Maximum 8 per entity, 64 total, 236 cells each so the row plus table padding fits the default 240-cell canvas. Attributes render before table constraints with a divider between nonempty sections. ER syntax whitespace is limited to ASCII space, tab and LF/CRLF. Composite FK preserves ordered mapping but does not infer a relationship or cardinality. Relationship endpoints use entity IDs and explicit Mermaid-style cardinality markers. Self, duplicate and disconnected relationships/entities are retained in source order.

Unsupported syntax fails rather than falling back: named `CONSTRAINT`, `DEFAULT`, `CHECK`, referential actions, inline table constraints, class/style/click, HTML/Markdown labels, Sequence/ER notes, advanced ER inheritance/weak entity/inferred cardinality, `RL`, `BT`.
