# 0.6 문법

## Header

첫 번째 유효 행은 다음 중 하나여야 합니다.

```text
flowchart LR
flowchart TD
flowchart TB
graph LR
graph TD
graph TB
```

## Nodes

```text
A
A[프로세스]
B{분기 조건}
C[(데이터 저장소)]
```

- ID는 ASCII letter 또는 `_`로 시작하며, 이후 ASCII letter, digit, `_`, `-`를 사용할 수 있습니다.
- ID는 최대 64 bytes입니다.
- label은 최대 96 terminal cells입니다.
- 동일 ID를 서로 다른 label이나 shape로 재정의하면 실패합니다.

## Edges

```text
A --> B
A -->|success| B
A -.->|failure| C
A --> B --> C
A --> B
B -.->|retry| A
```

- `-->`는 동기·정상 흐름에 사용합니다.
- `-.->`는 비동기·실패·보조 흐름에 사용합니다.
- cycle과 self-loop는 지원합니다. Feedback edge는 source edge order에 따라 결정적으로 선택되며 외곽 gutter와 `feedback:` legend로 표현됩니다.
- 중간 rank를 건너뛰는 forward edge는 외곽 gutter를 사용하며 label이 있으면 `routed:` legend에 표시됩니다.

## Subgraphs

```text
subgraph Service
  API --> Worker
end

subgraph Data[데이터 경계]
  Store[(Canonical store)]
end
```

- `subgraph ID`, `subgraph ID[label]`, `subgraph ID [label]`을 지원합니다.
- 최대 32개, 최대 중첩 깊이 8입니다.
- Node ID와 subgraph ID는 graph 전체에서 하나의 namespace를 공유합니다.
- 각 node는 root 또는 하나의 subgraph에 직접 소속됩니다. 기존 bare node를 다른 scope의 edge endpoint에서 참조해도 소속은 바뀌지 않습니다.
- 빈 leaf subgraph는 거부하지만 nonempty child만 가진 parent는 허용합니다.
- Scope를 포함한 문서에서 짝이 없는 `end`는 오류입니다. Flat 문서의 기존 `end`, `end[End]`, `end --> A` node 문법은 유지됩니다.
- Cross-subgraph edge는 frame-safe 외곽 route를 사용하며 label이 있으면 `routed:` legend에 표시됩니다.

## Sequence Diagrams

```text
sequenceDiagram
participant Client as Browser Client
participant API as API Gateway
participant Worker
Client ->> API: POST /events
API ->> Worker: enqueue
API -->> Client: 202 Accepted
Worker ->> Worker: record metrics
```

- Header는 정확히 `sequenceDiagram`이어야 합니다.
- `participant ID`, `participant ID as Label`을 지원하며 모든 participant를 message보다 먼저 선언합니다.
- Endpoint는 display label이 아니라 participant ID를 사용합니다.
- `->>`는 request, `-->>`는 return이며 source/target 순서에 따라 오른쪽·왼쪽 arrow를 렌더링합니다.
- 첫 `:` 뒤의 나머지 문자열이 message label입니다. 추가 `:`, `;`, `%%`는 label 문자로 보존합니다.
- 같은 sender의 연속 message가 fan-out이며 `A ->> B, C` 축약은 지원하지 않습니다.
- Self-message는 `A ->> A: label` 또는 `A -->> A: label`로 표현합니다.
- Participant ID와 display label은 각각 diagram 안에서 유일해야 합니다.
- 최소 1 participant와 1 message가 필요하며 최대 16 participants, 96 messages입니다.
- 긴 long-hop label은 전용 label row에서 중간 lifeline을 잠시 가릴 수 있습니다. Arrow row junction과 다음 row lifeline은 유지됩니다.

### Structured fragments

```text
loop 최대 3회
  Client ->> API: retry request
end

alt accepted
  API -->> Client: 202
else rejected
  API -->> Client: 400
end

opt audit
  Client ->> Client: record metrics
end
```

- `loop`, `opt`는 하나의 nonempty branch를 가집니다.
- `alt`는 정확히 하나의 `else`와 두 개의 nonempty branch를 가집니다.
- Fragment와 branch label은 keyword 뒤 나머지 문자열이며 최대 96 cells입니다.
- 최대 32 fragments, 중첩 깊이 8, message와 control을 합한 256 steps입니다.
- Message-only 입력은 기존 AST와 렌더링을 그대로 유지합니다. Fragment가 등장한 입력만 ordered step timeline으로 전환됩니다.
- Fragment frame row는 title 가독성을 위해 해당 row의 lifeline을 가릴 수 있습니다. 다음 row에서 lifeline은 복원됩니다.

### Explicit activation

```text
activate API
Client ->> API: request
API -->> Client: response
deactivate API
```

- `activate ID`는 현재 timeline boundary에서 participant의 solid activation bar를 시작합니다.
- `deactivate ID`는 participant별 LIFO top activation을 닫습니다.
- 같은 participant에서 최대 depth 8, diagram 전체 최대 96 activation starts입니다.
- Activate/deactivate 사이에는 적어도 하나의 message가 있어야 합니다.
- Activation pair는 `loop`·`alt/else`·`opt` 시작, branch, 종료 경계를 넘을 수 없습니다. 하나의 branch 안에서 시작하고 닫는 것은 허용합니다.
- Active message endpoint와 self-message rail은 가장 안쪽 activation bar에 붙습니다.
- Activation은 serialized diagram의 시각적 interval이며 실제 call stack의 증명이 아닙니다.

## Rejected input

- invalid UTF-8
- NUL, ESC, C0/C1 control, Unicode format/bidi control, ZWJ, variation selector
- 선행 결합 문자 또는 한 base 뒤 8개를 초과한 combining marks
- `classDef`, `style`, `click`, HTML/Markdown labels
- Sequence `par`/`and`, note와 ER diagrams, Flow 방향 `RL`, `BT`
