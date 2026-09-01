# 0.16 문법

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
- 인접 rank의 forward edge는 source-order tie-break를 가진 bounded median sweep과 edge별 lane을 사용합니다.
- 서로 다른 source와 서로 다른 target을 가진 route가 같은 cell component로 합쳐지면 topology를 추측하지 않고 실패합니다.
- 동일 endpoint의 parallel forward edge는 하나의 선으로 축약하지 않고 renderer에서 거부합니다. Feedback/outer route로 분리된 edge는 기존처럼 별도 route와 legend를 유지합니다.
- Unicode canvas는 N/E/S/W 연결 방향에서 corner·tee·junction을 합성합니다. `┼`는 실제 네 방향 연결일 때만 사용합니다.

## Viewport

- Standalone 기본은 240×200 cells, hard cap은 512×512 cells입니다.
- `-width`, `-height`로 성공 출력의 cell bounds를 제한합니다.
- `-fit`은 Flow의 요청 방향이 bounds를 넘을 때 반대 방향을 한 번 시도합니다. 두 방향이 모두 실패하면 clipping하거나 soft-wrap 가능한 출력을 내지 않고 오류를 반환합니다.
- Codex plugin wrapper는 120×200과 `-fit`을 기본 사용합니다.

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

### Parallel branch presentation

```text
par notify by email
  API ->> Email: send
and notify by sms
  API ->> SMS: send
and audit
  API ->> API: record
end
```

- `par`는 최소 하나의 `and`를 포함해 두 개 이상의 nonempty branch를 가집니다.
- 각 branch 내부 message 순서는 의미가 있지만, branch 사이의 세로/source 순서는 display order일 뿐 실행 순서나 happens-before를 뜻하지 않습니다.
- Renderer는 frame title에 `par (display order only)`를 항상 표시합니다.
- Activation pair는 하나의 `par` branch 안에서 완결되어야 하며 `and` 경계를 넘을 수 없습니다.

## ER Diagrams

```text
erDiagram
Customer ||--o{ Order : places orders
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
Audit[감사] {}
```

- Header는 정확히 `erDiagram`입니다.
- Entity는 `ID {` 또는 `ID[display label] {`로 열고 exact `}`로 닫습니다. Empty/disconnected entity도 허용합니다.
- Attribute는 `type name [PK] [FK] [UNIQUE] [NOT NULL]`을 지원합니다. Marker unit 순서는 자유지만 각 marker는 한 번만 허용하며 출력은 `PK FK UNIQUE NOT NULL type name` 순서입니다.
- `NOT NULL`은 인접한 두 token으로 이루어진 하나의 marker입니다. Marker는 대문자 exact form만 허용합니다.
- ER 구문의 공백 separator는 ASCII space, tab, LF/CRLF만 허용하며 NBSP·Unicode line/paragraph separator는 거부합니다.
- Attribute type/name은 ASCII ID이고 name은 entity 안에서 유일합니다. PK/FK/UNIQUE/NOT NULL은 표시 metadata이며 relationship, target 또는 다른 constraint를 자동 추론하지 않습니다.
- Multiline entity block은 다음 table constraint를 지원합니다. 각 column list는 2~8개이며 source order를 보존합니다.

```text
PRIMARY KEY (tenant_id, id)
UNIQUE (tenant_id, email)
FOREIGN KEY (tenant_id, customer_id) REFERENCES Customer(tenant_id, id)
```

- Table constraint keyword는 대문자 exact form입니다. ASCII space/tab은 괄호·comma 주변에서 허용하고 출력은 위 형식으로 정규화합니다.
- `PRIMARY`, `UNIQUE`, `FOREIGN`은 multiline entity body의 첫 token에서 table-constraint keyword로 예약됩니다.
- Local attribute와 FK target entity/attribute는 뒤에 선언할 수 있으며 EOF에서 해소합니다. Self-reference도 허용합니다.
- Entity마다 table constraint 최대 8개, diagram 전체 64개, constraint text 최대 236 cells입니다. 좌우 table padding을 포함해 기본 240-cell canvas에 들어갑니다.
- Table PRIMARY KEY는 entity당 하나이며 attribute PK와 혼용하지 않습니다. Composite FK의 local/target column 수는 같아야 합니다.
- Composite FK는 relationship·cardinality·attribute FK marker를 자동 생성하지 않습니다. 별도 relationship은 별도 source evidence가 있어야 합니다.
- Named `CONSTRAINT`, `DEFAULT`, `CHECK`, referential actions, inline table constraint는 지원하지 않습니다.
- Relationship은 `From <left-marker>--<right-marker> To : label` 형식입니다. Entity block보다 앞에 쓸 수 있지만 EOF까지 모든 endpoint block이 명시되어야 합니다.
- Left marker: `o|`=0..1, `||`=1, `}o`=0..N, `}|`=1..N.
- Right marker: `|o`=0..1, `||`=1, `o{`=0..N, `|{`=1..N.
- 첫 `:` 뒤 나머지가 필수 relationship label입니다. Self·duplicate·reverse relationship을 source order로 보존합니다.
- 최대 32 entities, 64 relationships, attributes 총 192/entity당 32입니다.
- Renderer는 attributes 다음에 table constraints를 source order로 표시하고 두 section이 모두 있으면 divider를 둡니다. Explicit relationship만 endpoint cardinality marker, source-order rails와 `relationships:` legend를 만듭니다.

## State Diagrams

```text
stateDiagram-v2
direction TD
state "검증 중" as Validating
state Committing
state CommitOutcome <<choice>>
state Backoff
state Acked
[*] --> Validating
Validating --> Committing : valid
Committing --> CommitOutcome : commit result
CommitOutcome --> Backoff : [transient failure and attempt below 3]
CommitOutcome --> Acked : [commit succeeds]
Backoff --> Committing : retry
Acked --> [*]
policy Backoff --> Committing : retry :: retry "attempt below 3"
```

- Header는 exact `stateDiagram-v2`입니다. Optional `direction TD|LR`은 header 다음, 첫 declaration/transition 전에 한 번만 허용합니다. 기본 방향은 TD입니다.
- State declaration은 `state ID` 또는 `state "display label" as ID`입니다. ID와 display label은 각각 diagram 안에서 유일해야 합니다.
- Declaration과 transition은 섞어 쓸 수 있으며 concrete endpoint는 EOF까지 explicit declaration이 있어야 합니다. Endpoint로 state를 자동 생성하지 않습니다.
- Concrete transition은 `A --> B`, `A --> B : event`, `A --> B : event [guard]`입니다. Colon이 있으면 nonempty event가 필요하고 guard는 하나의 trailing nonempty bracket입니다.
- `[*] --> State` initial은 정확히 하나 필요합니다. `State --> [*]` final은 0개 이상 허용하며 pseudo transition에는 event/guard를 붙이지 않습니다.
- Exact duplicate transition은 거부하고 event/guard가 다른 transition은 source order로 보존합니다. Self/cycle도 명시 source order로 유지합니다.
- Choice declaration은 exact `state ID <<choice>>` 또는 `state "display label" as ID <<choice>>`입니다. `Choice`, `Decision` 같은 ID/label이나 ordinary state의 다중 outbound를 choice로 승격하지 않습니다.
- Choice는 ordinary state에서 정확히 하나의 guard 없는 inbound와 서로 다른 ordinary target으로 2~8개의 guard-only outbound `Choice --> Target : [guard]`를 가져야 합니다. Choice-to-choice, self, initial/final 직접 연결은 거부합니다.
- Choice guard는 ASCII space/tab을 양끝에서 제거해 canonicalize하고 같은 choice 안에서 exact unique여야 합니다. Guard의 의미적 상호배타성·우선순위·default·완전성은 검증하거나 추론하지 않습니다.
- Guard-only transition은 choice outbound에만 허용합니다. Choice incident transition에는 retry/timeout/compensation policy를 붙일 수 없습니다.
- Transition policy는 `policy <exact labeled concrete transition> :: <kind> "detail"` 형식입니다. Kind는 exact `retry`, `timeout`, `compensation` 세 개이며 policy statement는 참조하는 transition보다 앞에 올 수 있습니다.
- Policy는 endpoint·event·guard가 모두 같은 기존 transition을 EOF에서 참조합니다. Pseudo/unlabeled/missing transition, 같은 transition의 같은 kind 중복, unquoted·empty detail은 거부합니다. 한 transition에 서로 다른 kind는 source order로 허용합니다.
- Ordinary transition event/guard의 quote는 기존 호환성을 위해 허용하지만, quoted label은 policy separator와 구분할 수 없으므로 policy target으로 참조할 수 없습니다.
- Policy는 renderer metadata이며 state·transition·initial/final·feedback 분류를 생성하거나 바꾸지 않습니다. Detail은 source에서 직접 확인된 정책 원문이고 duration·attempt·backoff·rollback 보장을 계산하지 않습니다.
- 최대 32 total ordinary/choice states, choice당 8 branches, 64 transitions, 64 policies, ID 64 bytes, state label·canonical `event [guard]`/`[guard]`·policy detail 각각 96 cells입니다.
- Renderer는 state box 사이에 bounded connector lane을 예약합니다. Cycle/self는 reachability로 분류해 `feedback:` legend에, 나머지는 `transitions:` legend에 source order로 표시합니다.
- Transition label의 policy-like suffix는 backward-compatible ordinary event text이며 policy로 해석하지 않습니다.
- Composite/nested state, fork/join/history, note/style, concurrency와 이름 기반 choice/policy 승격은 지원하지 않습니다.

## Rejected input

- invalid UTF-8
- NUL, ESC, 구조 whitespace인 LF·tab·CRLF을 제외한 C0/C1 control, Unicode format/bidi control, ZWJ, variation selector
- ER syntax separator 위치의 NBSP와 기타 Unicode whitespace
- 렌더되는 label에서 선행 결합 문자 또는 한 base 뒤 8개를 초과한 combining marks
- `classDef`, `style`, `click`, HTML/Markdown labels
- Sequence/ER note, ER relationship attributes·inheritance·weak entity·inferred cardinality, Flow 방향 `RL`, `BT`
- State composite/fork/join/history/note/style과 exact subset 밖의 state 문법
