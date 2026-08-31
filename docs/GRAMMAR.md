# v0.1 문법

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
```

- `-->`는 동기·정상 흐름에 사용합니다.
- `-.->`는 비동기·실패·보조 흐름에 사용합니다.
- cycle은 v0.1에서 지원하지 않습니다.

## Rejected input

- invalid UTF-8
- NUL, ESC, C0/C1 control, Unicode format/bidi control, ZWJ, variation selector
- 선행 결합 문자 또는 한 base 뒤 8개를 초과한 combining marks
- `classDef`, `style`, `click`, `subgraph`, HTML/Markdown labels
- sequence/ER diagrams와 방향 `RL`, `BT`
