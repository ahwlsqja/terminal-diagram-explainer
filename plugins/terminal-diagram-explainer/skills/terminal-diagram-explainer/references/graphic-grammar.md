# Safe graphical Mermaid grammar

`render_diagram`은 Mermaid 11 표준 source를 sandboxed UI에서 렌더한다. Fact evidence 규칙과 요소 상한은 `SKILL.md`가 소유하며 이 문서는 graphical backend와 교차되는 문법만 정의한다.

## Flowchart

```text
flowchart LR|TD
subgraph ScopeID[scope label]
NodeID[process]
DecisionID{decision}
StoreID[(data store)]
A -->|normal| B
A -.->|failure or async| C
end
```

## Sequence

```text
sequenceDiagram
participant Client as Browser Client
participant API as API Gateway
Client->>API: request
API-->>Client: response
loop label
  Client->>API: retry
end
```

`alt/else/end`, `opt/end`, `par/and/end`, `activate/deactivate`는 direct evidence gate를 통과할 때만 사용한다.

## ER

```text
erDiagram
Customer ||--o{ Order : references
Customer {
  uuid id PK
  string email UK
}
Order {
  uuid id PK
  uuid customer_id FK
}
```

Graphical Mermaid에는 entity display alias, `PRIMARY KEY (...)`, `UNIQUE (...)`, `FOREIGN KEY (...) REFERENCES ...` custom statement를 넣지 않는다. Composite constraint의 ordered columns는 해설에서 전달한다.

## State

```text
stateDiagram-v2
direction TD
state "Validating" as Validating
state Choice <<choice>>
[*] --> Validating
Validating --> Choice : result
Choice --> Done : [accepted]
Done --> [*]
```

Graphical Mermaid에는 `policy ... :: retry|timeout|compensation` custom statement를 넣지 않는다. 확인된 policy detail은 transition label에 사실을 왜곡하지 않는 범위로 축약하거나 해설에서 전달한다.

## Forbidden

- `click`, external image/icon metadata, `href`/remote link
- `url()`, `@import`, `themeCSS`, `%%{init...}%%`
- active HTML과 entity-encoded HTML
- terminal control, bidi override, zero-width format characters

Formatting-only `<br>`와 `<i>`는 renderer policy상 허용되지만 Skill은 plain one-line label을 기본으로 한다.
