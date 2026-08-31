# 0.4 문법

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

## Rejected input

- invalid UTF-8
- NUL, ESC, C0/C1 control, Unicode format/bidi control, ZWJ, variation selector
- 선행 결합 문자 또는 한 base 뒤 8개를 초과한 combining marks
- `classDef`, `style`, `click`, HTML/Markdown labels
- sequence/ER diagrams와 방향 `RL`, `BT`
