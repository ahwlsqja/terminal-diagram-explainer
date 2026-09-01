---
name: terminal-diagram-explainer
description: 비자명한 소프트웨어 아키텍처·데이터 흐름·API·Worker 호출 순서·상태 전이·장애 원인을 한 줄 결론, bounded 터미널 Flowchart 또는 Sequence Diagram, 단계별 해설로 설명한다. 관계·분기·경계·시간 순서가 여러 개인 개발 설명과 코드 변경의 런타임 의미를 전달할 때 사용하며 단순한 한 단계 답변이나 text-only 요청에는 사용하지 않는다.
---

# Terminal Diagram Explainer

복잡한 소프트웨어 동작을 서버·코어 개발자가 한 번에 파악할 수 있는 설명으로 바꾼다. 이 Skill은 **표현 계층**일 뿐이며 프로젝트 규칙이나 구현 권한을 바꾸지 않는다.

## 경계

- 이 Skill 때문에 프로젝트의 `AGENTS.md`, SDLC, workflow, repo-local Skill 또는 source를 생성·수정하지 않는다.
- 사용자의 별도 구현 요청이 없다면 읽고 설명만 한다.
- repo-local 규칙, security boundary, source of truth가 항상 우선이다.
- secret, credential, PII, 내부 hostname의 원문을 다이어그램 label, Mermaid source, 최종 답변 또는 내부 fact ledger에 복제하지 않는다. 위치·식별자명만 기록하고 값은 `[REDACTED]`로 표시한다.
- 관계·분기·경계가 3개 미만이거나 도식이 이해를 높이지 않으면 일반 텍스트로 답한다.

## 사실 확인

현재 구현을 설명할 때는 가장 가까운 `AGENTS.md`와 repo의 SSoT 라우팅을 따른다. 운영 사실은 code/config/workflow/API의 실제 호출 경로로 확인하고, 확인되지 않은 컴포넌트나 데이터 흐름은 사실처럼 그리지 않는다.

도식을 만들기 전에 내부 fact ledger를 만든다. 각 diagram label과 최종 답변의 강한 주장(component, owner, relationship, cardinality, retry, ordering, security guarantee)을 확인한 source fact에 연결한다. 이 ledger는 평가·검증용이며 기본 답변에는 노출하지 않는다.

- Identifier와 함수 이름은 evidence가 아니다. `runParallel`, `activateFeature`, `customer_id` 같은 이름만으로 concurrency, activation lifetime, FK relationship을 만들지 않는다.
- 확인된 사실과 추론을 분리한다. 추론이 필요하면 `가능성`, `확인 필요`, `제공된 코드만으로는 알 수 없음`처럼 불확실성을 표시한다.
- Required fact가 부족하면 작은 부분 흐름만 설명하거나 도식을 생략한다. 빈칸을 일반적인 architecture pattern으로 채우지 않는다.

## 기본 출력

1. **한 줄 결론**: 무엇이 왜 그렇게 동작하는지 먼저 말한다.
2. **터미널 도식 한 개**: 한 가지 핵심 이야기만 5~12 nodes 또는 최대 6 participants로 표현한다.
3. **읽는 순서**: 도식 label과 연결한 3~7단계 설명을 쓴다.
4. **개발 핵심**: 관련 있는 항목만 고른다.
   - source of truth와 data ownership
   - sync/async boundary와 state transition
   - validation, retry, idempotency, ordering
   - failure path와 observability
   - trust/tenant/security boundary
5. **구체 예시**: 입력 하나가 각 경계를 통과해 어떤 결과가 되는지 보여준다.
6. repo 분석 결과라면 중요한 주장을 실제 file:line에 연결한다.

## 도식 선택

- 처리·데이터 파이프라인: `flowchart LR`
- 분기·의사결정·장애 처리: `flowchart TD`
- API request/response, 서비스 간 호출 순서, fan-out, self-call: `sequenceDiagram`
- Entity ownership, table attributes, PK/FK, cardinality 관계: `erDiagram`
- 정상 흐름은 `-->`, 실패·비동기·보조 흐름은 `-.->`를 사용한다.
- Node/participant/entity ID는 source 의미를 드러내는 짧은 semantic identifier를 사용한다. Cycle·routed·relationship legend에는 ID가 그대로 표시되므로 `A`, `B`, `N1` 같은 opaque ID를 쓰지 않는다.
- decision은 `ID{label}`, data store/view는 `ID[(label)]`로 표시한다.
- ownership, service, data, trust boundary가 설명의 핵심이면 `subgraph ID[label] ... end`로 묶는다. Node ID와 subgraph ID는 전체 graph에서 유일해야 한다.
- cycle과 self-loop를 지원한다. Feedback edge는 외곽 route로 그리고 label은 도식 아래 `feedback:` legend에 표시된다.
- 중간 rank를 건너뛰는 edge도 외곽 route를 사용하며 label은 `routed:` legend에 표시될 수 있다.
- cross-subgraph edge는 frame-safe 외곽 route를 사용하며 label은 `routed:` legend에 표시될 수 있다.
- Sequence participant는 먼저 명시 선언하고, request는 `->>`, return은 `-->>`, fan-out은 연속 message로 표시한다.
- 재시도·반복 구간은 `loop label ... end`, 상호 배타적 결과는 `alt label ... else label ... end`, 선택적 호출은 `opt label ... end`로 묶는다.
- 각 fragment branch에는 실제 message를 넣고, frame label은 조건·반복 이유를 짧게 쓴다.
- Source에서 participant의 active lifetime이 확인되고 설명 가치가 있을 때만 `activate ID ... deactivate ID`를 사용한다.
- Activation pair 안에는 message를 넣고 fragment 경계를 넘기지 않는다. Activation을 실제 call stack의 증거처럼 해석하지 않는다.
- 실제로 독립 실행 가능한 branch를 함께 보여줄 때만 `par label ... and label ... end`를 사용한다.
- `par` branch의 source/display order를 실행 순서나 happens-before로 설명하지 않는다. 각 branch 내부 순서와 frame 전후 경계만 순서 의미를 가진다.
- ER entity는 `ID[display label] { ... }`, attribute는 `type name [PK] [FK] [UNIQUE] [NOT NULL]`, relationship은 cardinality를 생략하지 않고 명시한다.
- 2개 이상 ordered columns가 직접 명시되면 multiline entity 안에 `PRIMARY KEY (...)`, `UNIQUE (...)`, `FOREIGN KEY (...) REFERENCES Entity(...)`를 source column order 그대로 쓴다.
- Composite FK의 local columns, target entity, target columns는 DDL·ORM table constraint의 직접 evidence가 모두 있을 때만 표시한다. Field/index 이름이나 같은 이름의 column 집합으로 mapping을 완성하지 않는다.
- Composite FK만으로 ER relationship·cardinality·business label을 추가하지 않는다. Relationship line은 별도의 direct schema contract가 있을 때만 그린다.
- Lifecycle의 explicit states와 source→target transition이 핵심이면 `stateDiagram-v2`를 사용한다. Status enum·상수 목록만 있으면 state 후보일 뿐 transition evidence가 아니므로 state diagram을 만들지 않는다.
- 내부 state ledger에서 각 box를 explicit state fact에 연결하고 각 edge를 source state·target state·event fact에 연결한다. Event, method, command, result 단어를 별도 state로 승격하지 않는다.
- `DLQ -- publish succeeds --> Acked`처럼 event가 두 state 사이에 주어지면 `DLQ --> Acked : publish succeeds`로 표시한다. `publish succeeds`나 `published`를 state로 만들지 않는다.
- Initial은 bootstrap contract, final은 terminal contract가 직접 확인될 때만 `[*]`로 표시한다. `Done`, `Failed`, `isTerminal`, `transitionToRetry` 같은 이름은 증거가 아니다.
- Event는 해당 transition을 발생시키는 trigger가, guard는 그 transition을 실제로 제어하는 조건이 source에 있을 때만 붙인다. Alias도 명시 display mapping이 있을 때만 사용한다.
- `retry`, `timeout`, `compensate`라는 event label을 source에서 확인해 보존할 수는 있지만 attempt·deadline·backoff·보상 보장을 일반 관례로 확장하지 않는다.
- 명시 DDL·ORM schema의 attribute나 constraint가 설명 핵심이면 relationship이 없어도 단일 entity ER table을 사용할 수 있다. 존재하지 않는 relationship은 추가하지 않는다.
- Field 이름만 있고 schema type·constraint·relation 근거가 없으면 `unknown` attribute를 채운 ER table을 만들지 말고 text-only로 evidence gap을 설명한다.
- FK marker는 표시 metadata로만 사용한다. Source에서 참조 target·integrity가 확인되지 않았다면 relationship을 추론해 추가하지 않는다.
- UNIQUE와 NOT NULL은 DDL·ORM schema constraint 또는 명시 schema contract에 직접 존재할 때만 표시한다. `email`, `is_unique`, `required`, non-pointer type, PK 관례, 애플리케이션 중복 검사는 constraint evidence가 아니다.
- PK에서 NOT NULL을, UNIQUE에서 business identity를 자동 유도하지 않는다. 직접 근거가 없으면 marker를 생략한다.
- ER relationship label도 evidence가 필요하다. DDL의 `REFERENCES`만 확인되면 `references`처럼 source에 있는 중립 용어를 사용하고, `owns`, `has`, `places` 같은 business verb를 만들지 않는다.
- Strong notation은 direct evidence gate를 통과해야 한다: `par`=동시성 primitive·독립 branch, activation=participant lifetime boundary, PK/FK/UNIQUE/NOT NULL=DDL·ORM schema constraint, cardinality=명시 schema/contract. 이름·관례·일반적인 설계는 gate를 통과시키지 않는다.
- Sequence는 호출 시간 순서가 핵심일 때만 사용한다. Ownership·분기·데이터 이동이 핵심이면 Flowchart를 유지한다.
- 현재 버전은 named constraint, DEFAULT/CHECK/action, inline table constraint, state composite/fork/join/history/note/style, class/style/click, Sequence/ER note, advanced ER inheritance·weak entity·inferred cardinality를 지원하지 않는다.

문법이 필요하면 [references/grammar.md](references/grammar.md)를 읽는다. 설명 관점과 예시가 필요하면 [references/developer-lenses.md](references/developer-lenses.md)를 읽는다.

## 렌더링

Mermaid subset source를 만든 뒤 이 Skill 디렉터리의 `scripts/render.sh`에 stdin으로 전달한다.

```bash
printf '%s\n' "$diagram_source" | scripts/render.sh
```

- 성공한 renderer 출력을 `text` code fence에 넣는다.
- Renderer 성공 stdout은 그대로 사용하고 line·glyph·legend를 수동 편집하지 않는다. Session 내부에는 source, exit status, stderr, output dimensions을 검증 evidence로 유지한다.
- Mermaid source는 사용자가 재사용을 요청했을 때만 함께 보여준다.
- 실패하면 오류가 가리키는 문법·limit을 줄여 한 번만 재시도한다.
- 두 번째 실패 시 작은 수동 Unicode 도식으로 fallback하고 실패를 한 문장으로 밝힌다.
- renderer는 프로젝트 파일을 만들거나 자동 다운로드·업데이트하지 않는다.
