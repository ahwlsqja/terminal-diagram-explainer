# 개발자 설명 렌즈

아래 항목은 사실 목록이 아니라 **확인할 관점**이다. 실제 요청마다 source와 SSoT로 존재 여부를 검증한다.

## Evidence gate

- 이름은 의미의 증거가 아니다. `parallel`, `activate`, `owner`, `*_id` 같은 token은 구현·schema·runtime contract로 확인한다.
- 도식의 모든 component와 edge는 source fact가 있어야 한다. Unknown sender, broker, retry, cache, transaction, FK target은 자동 보완하지 않는다.
- Renderer legend가 ID를 노출하는 cycle·routed·relationship 경로에서는 opaque ID 대신 의미 있는 source-derived ID를 사용한다.
- `par`, activation, PK/FK/cardinality처럼 강한 표기는 직접 evidence가 없으면 사용하지 않는다.
- 근거가 부족한 요청은 확인된 두세 단계만 설명하거나 text-only로 답한다.

## 요청·데이터 흐름

- producer가 어떤 identity·scope·payload를 만든다.
- ingress 경계에서 무엇을 validate·normalize·enrich한다.
- raw fact, canonical model, read model의 ownership이 어디다.
- duplicate, late, out-of-order 입력을 어떻게 취급한다.
- consumer가 authoritative read boundary를 우회하지 않는지 확인한다.

```text
flowchart LR
Client[Caller] --> Ingress{Validate}
Ingress -->|valid| Model[(Canonical model)]
Ingress -.->|invalid| Reject[Reject + observe]
Model --> Consumer[API or worker]
```

## API·서비스 경계

- authentication과 authorization을 분리한다.
- trust/tenant boundary와 data ownership을 표시한다.
- computed field를 어느 계층이 소유하는지 확인한다.
- DB transaction과 external call의 경계를 표시한다.
- cache, staleness, error contract가 consumer에게 어떻게 보이는지 설명한다.

호출의 시간 순서와 request/response가 핵심이면 Flowchart 대신 작은 Sequence Diagram을 사용한다.

```text
sequenceDiagram
participant Client
participant API
participant Store
Client ->> API: request
API ->> Store: canonical read
Store -->> API: result
API -->> Client: response
```

Retry나 결과별 응답 차이가 핵심이면 전체 호출을 복제하지 않고 `loop` 또는 `alt/else` frame으로 해당 message 구간만 묶는다.

Participant가 실제로 처리 중인 구간이 source에서 확인되면 `activate`/`deactivate`로 표시할 수 있다. 단, 이를 call stack 보장으로 확대 해석하지 않고 fragment branch 안에서 pair를 완결한다.

독립 branch를 `par/and`로 묶을 때는 branch 내부 순서만 설명한다. Branch의 화면상 위아래 순서를 실제 실행 순서나 happens-before로 해석하지 않는다.

Entity·table ownership과 cardinality가 핵심이면 ER Diagram을 사용한다. PK/FK marker와 relationship은 source에서 각각 확인하고, FK 이름만 보고 target 관계를 추론하지 않는다.

명시 schema의 attribute·constraint 자체가 핵심이면 관계가 없는 단일 entity ER도 유효하다. 반대로 field 이름만 있고 type·constraint·relation source가 없으면 ER을 만들지 않는다.

ER relationship label은 schema·annotation·query에서 확인된 용어를 사용한다. DDL `REFERENCES`만 있으면 `references`로 표시하고 business ownership verb를 새로 만들지 않는다.

UNIQUE와 NOT NULL은 DDL·ORM schema constraint 또는 명시 schema contract에서 각각 확인한다. Field 이름, 언어 type, validation code, 중복 선조회는 DB constraint를 보장하지 않는다.

Composite PK/UNIQUE/FK는 ordered local column list를 그대로 보존한다. FK target entity와 target column list까지 직접 확인하고, column 이름이 비슷하다는 이유로 mapping이나 cardinality를 만들지 않는다.

Lifecycle은 state 목록보다 transition contract를 먼저 확인한다. Source state, target state, trigger, guard, bootstrap, terminal 근거를 각각 확인하고 enum·함수 이름으로 빈칸을 채우지 않는다.

Event·command·method·result는 transition label 후보이지 state 후보가 아니다. State로 명시된 명사만 box로 만들고 event를 state로 승격하지 않는다.

Choice는 지속 state가 아니라 명시적 runtime decision point다. Decision contract, 정확히 하나의 inbound, branch guard와 target을 각각 확인하고 `Choice`, `Decision`, `shouldChoose` 같은 이름으로 빈칸을 채우지 않는다.

- Choice branch guard는 source text 그대로 보존하고 source order는 display order로만 다룬다.
- 상호배타성, 우선순위, first-match, default/else, exhaustive coverage는 각각 별도 direct evidence가 없으면 주장하지 않는다.
- Choice incident transition에는 retry/timeout/compensation policy를 붙이지 않는다. Policy가 필요하면 choice 전후 ordinary-state transition에서 별도 contract를 확인한다.

Retry·timeout·compensation policy는 event 이름과 별개로 검증한다. Exact source→target transition과 event/guard 외에 policy kind와 detail의 direct runtime contract가 있어야 별도 policy statement를 만든다.

- Retry: attempt limit, backoff·jitter, 재진입 target은 각각 확인된 값만 보존한다. 하나를 확인했다고 나머지를 채우지 않는다.
- Timeout: deadline/timer 기준과 source→target edge가 직접 있어야 한다. `onTimeout` 이름으로 duration, clock origin, cancel/reset, terminal을 만들지 않는다.
- Compensation: trigger와 action, 성공/실패 후 state를 각각 확인한다. Action 이름만으로 전체 rollback, 성공, 원자성, idempotency를 주장하지 않는다.
- Policy는 기존 transition의 metadata이며 state, edge, initial/final 또는 feedback 분류를 추가하지 않는다.

## Worker·비동기 처리

- enqueue, ack, commit 시점을 구분한다.
- retry 조건, maximum attempts, idempotency key를 표시한다.
- poison message와 terminal failure의 관측 경로를 표시한다.
- deploy/migration compatibility가 runtime보다 먼저 충족되는지 설명한다.

## 장애·변경 설명

- before/after를 한 화면에 억지로 합치지 않는다. 핵심 변화가 하나면 변경 후 흐름 하나와 차이 3개로 설명한다.
- 위험 지점은 정확히 특정하고 증거와 추론을 구분한다.
- ordering, first-match, sort/reverse가 runtime 의미를 바꾸면 edge 순서와 설명에 함께 드러낸다.
