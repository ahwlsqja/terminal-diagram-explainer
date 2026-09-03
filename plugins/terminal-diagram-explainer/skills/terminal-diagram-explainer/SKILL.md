---
name: terminal-diagram-explainer
description: 비자명한 소프트웨어 아키텍처·데이터 흐름·API·Worker 호출 순서·상태 전이·장애 원인을 한 줄 결론, interactive Mermaid UI 또는 bounded fallback Flowchart·Sequence·ER·State Diagram, 단계별 해설로 설명한다. 관계·분기·경계·시간 순서가 여러 개인 개발 설명과 코드 변경의 런타임 의미를 전달할 때 사용하며 단순한 한 단계 답변이나 text-only 요청에는 사용하지 않는다.
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
2. **그래픽 도식 한 개**: 한 가지 핵심 이야기만 5~12 nodes 또는 최대 6 participants로 표현한다. `render_diagram` MCP tool을 기본으로 사용하고 UI를 지원하지 않는 surface에서만 artifact/terminal fallback을 사용한다.
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
- 정상 흐름은 `-->`, 실패·비동기·보조 흐름은 `-.->`를 사용한다. Edge label은 종류와 무관하게 반드시 pipe 표준 문법 `A -->|label| B`, `A -.->|label| B`를 사용한다. `A -- "label" --> B`, `A -- "label" -.-> B`처럼 quoted infix와 화살표를 조합하지 않는다.
- Node/participant/entity ID는 source 의미를 드러내는 짧은 semantic identifier를 사용한다. Cycle·routed·relationship legend에는 ID가 그대로 표시되므로 `A`, `B`, `N1` 같은 opaque ID를 쓰지 않는다.
- decision은 `ID{label}`, data store/view는 `ID[(label)]`로 표시한다.
- ownership, service, data, trust boundary가 설명의 핵심이면 `subgraph ID[label] ... end`로 묶는다. Node ID와 subgraph ID는 전체 graph에서 유일해야 한다.
- cycle과 self-loop를 지원한다. Feedback edge는 외곽 route로 그리고 label은 도식 아래 `feedback:` legend에 표시된다.
- 중간 rank를 건너뛰거나 혼합 fan-out/fan-in junction을 만드는 edge는 외곽 route를 사용하며, label 유무와 무관하게 semantic ID endpoint가 `routed:` manifest에 표시된다.
- cross-subgraph edge는 endpoint의 최소 공통 조상 frame을 벗어나지 않는 corridor를 사용하며, semantic ID endpoint가 `routed:` manifest에 표시된다.
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
- Runtime이 하나의 decision point에서 guard로 다음 lifecycle state를 선택한다는 직접 contract가 있을 때만 `state ID <<choice>>` 또는 alias choice를 사용한다. Choice/Decision 이름, enum, 함수, 다중 outbound, guard 존재만으로 choice를 만들지 않는다.
- Choice마다 ordinary state에서 정확히 하나의 inbound와 서로 다른 ordinary target으로 2~8개의 `: [guard]` branch를 직접 확인한다. Choice-to-choice·self·initial/final 직접 연결이나 choice incident policy를 만들지 않는다.
- Choice guard는 확인된 source text만 보존한다. Guard 순서를 실행 우선순위로 설명하거나 상호배타성·default·else·완전성을 일반 관례로 채우지 않는다.
- Choice는 presentation상 inbound source 뒤, branch target 앞에 선언해 흐름을 읽기 쉽게 한다. 이 declaration/source order를 guard 우선순위나 runtime evaluation order라고 설명하지 않는다.
- `retry`, `timeout`, `compensate`라는 event·함수·enum 이름은 transition policy evidence가 아니다. Policy kind와 detail이 별도 runtime contract로 직접 확인될 때만 `policy <exact labeled transition> :: <retry|timeout|compensation> "detail"`을 사용한다.
- Policy statement는 endpoint·event·guard가 모두 같은 기존 transition을 그대로 반복한다. Detail에는 source가 직접 명시한 attempt limit·backoff·deadline 기준·compensation action만 보존하고 미확인 값을 채우지 않는다.
- Policy는 transition metadata일 뿐이다. Retry policy에서 새 loop/backoff state를, timeout policy에서 timer/final state를, compensation policy에서 inverse edge·성공·전체 rollback·원자성·idempotency를 자동 생성하거나 보장하지 않는다.
- 명시 DDL·ORM schema의 attribute나 constraint가 설명 핵심이면 relationship이 없어도 단일 entity ER table을 사용할 수 있다. 존재하지 않는 relationship은 추가하지 않는다.
- Field 이름만 있고 schema type·constraint·relation 근거가 없으면 `unknown` attribute를 채운 ER table을 만들지 말고 text-only로 evidence gap을 설명한다.
- FK marker는 표시 metadata로만 사용한다. Source에서 참조 target·integrity가 확인되지 않았다면 relationship을 추론해 추가하지 않는다.
- UNIQUE와 NOT NULL은 DDL·ORM schema constraint 또는 명시 schema contract에 직접 존재할 때만 표시한다. `email`, `is_unique`, `required`, non-pointer type, PK 관례, 애플리케이션 중복 검사는 constraint evidence가 아니다.
- PK에서 NOT NULL을, UNIQUE에서 business identity를 자동 유도하지 않는다. 직접 근거가 없으면 marker를 생략한다.
- ER relationship label도 evidence가 필요하다. DDL의 `REFERENCES`만 확인되면 `references`처럼 source에 있는 중립 용어를 사용하고, `owns`, `has`, `places` 같은 business verb를 만들지 않는다.
- Strong notation은 direct evidence gate를 통과해야 한다: `par`=동시성 primitive·독립 branch, activation=participant lifetime boundary, PK/FK/UNIQUE/NOT NULL=DDL·ORM schema constraint, cardinality=명시 schema/contract, choice=explicit decision point·guarded branch contract, transition policy=explicit kind·detail runtime contract. 이름·관례·일반적인 설계는 gate를 통과시키지 않는다.
- Sequence는 호출 시간 순서가 핵심일 때만 사용한다. Ownership·분기·데이터 이동이 핵심이면 Flowchart를 유지한다.
- Transition label의 policy-like text는 ordinary event로만 보존하며 policy로 승격하지 않는다.
- 현재 버전은 named constraint, DEFAULT/CHECK/action, inline table constraint, state composite/fork/join/history/note/style, class/style/click, Sequence/ER note, advanced ER inheritance·weak entity·inferred cardinality를 지원하지 않는다.

문법이 필요하면 [references/grammar.md](references/grammar.md)를 읽는다. 설명 관점과 예시가 필요하면 [references/developer-lenses.md](references/developer-lenses.md)를 읽는다.

## 렌더링

확인된 fact ledger에서 Mermaid 11 표준 source를 만든 뒤 다음 우선순위를 적용한다.

1. `render_diagram` MCP tool이 있으면 `{source, title, theme: "auto"}`로 호출한다. 이 결과의 interactive UI가 기본 도식이다.
2. Tool은 Flowchart·Sequence·ER·State의 표준 Mermaid source만 받는다. `policy` custom statement, ER display alias/table constraint처럼 표준 Mermaid가 아닌 metadata는 source에 넣지 말고 단계별 해설에 보존한다.
3. Codex CLI/TUI는 MCP Apps iframe을 inline으로 표시하지 않으며 image block도 `<image content>` placeholder로만 보일 수 있다. 실제 PNG가 보이면 그것을 기본 도식으로 사용한다. Placeholder만 보이면 tool result의 `Local interactive HTML` HTTP link를 제공하고, `structuredContent.terminalFallback`을 다시 출력하지 않는다. PNG가 생성되지 않았을 때만 terminal fallback을 최종 답변의 `text` code fence로 보여준다. `/app`으로 같은 세션을 Desktop App에서 열 수도 있다고 안내하되 TUI에서 "위에 interactive diagram이 보인다"고 말하지 않는다.
4. Desktop App처럼 MCP Apps UI를 실제로 표시하는 host에서는 terminal preview를 최종 답변에 중복하지 않는다.
5. Tool이 없거나 `terminalFallback`이 비어 있고 UI도 표시되지 않는 surface에서만 아래 artifact fallback을 실행한다.
6. Artifact 변환기도 없을 때만 terminal text renderer를 직접 사용한다.

그래픽 source의 안전한 표준 문법은 [references/graphic-grammar.md](references/graphic-grammar.md)를 따른다. `click`, external image/icon, `url()`, `@import`, init directive, `themeCSS`, active HTML은 만들지 않는다.

- 사용자가 source 재사용을 명시적으로 요청하지 않은 한 raw Mermaid source나 `mermaid`/`flowchart` code fence를 최종 답변에 출력하지 않는다.
- Label은 한 줄 plain text로 유지한다. `<br/>`·HTML·Markdown으로 줄바꿈이나 스타일을 넣지 말고 짧은 label 또는 여러 node로 분리한다.

```bash
printf '%s\n' "$diagram_source" | scripts/render-artifacts.sh
```

- MCP UI가 실제로 표시된 경우 PNG·HTML을 중복 첨부하지 않는다. Tool call 성공만으로 UI 표시 성공을 추정하지 않는다. Tool result에 PNG image block이 있으면 그 결과를 우선하고, 별도 artifact fallback을 실행한 경우에만 stdout JSON의 `png` 경로를 `view_image` 같은 local image 도구로 읽어 image block으로 첨부한다.
- 같은 JSON의 `html` 절대 경로를 `Interactive HTML로 열기` Markdown 링크로 함께 제공해 사용자가 pan·zoom·fit viewer를 선택할 수 있게 한다. `renderer`가 `mermaid-cli`인지 확인하고 `terminal-svg`면 최종 bounded fallback임을 내부 evidence에 기록한다. 사용자가 inline image만 요청하면 HTML 링크를 생략할 수 있다.
- Feedback/cycle이 2개 이상이거나 node가 8개 이상인 복잡한 도식은 PNG 미리보기와 HTML 링크를 모두 제공한다. 단순 도식은 PNG를 우선한다.
- SVG/XML source나 PNG 경로 문자열을 최종 답변 본문에 붙이지 않는다.
- Renderer가 만든 SVG geometry·PNG·HTML을 수동 편집하지 않는다. Session 내부에는 source, exit status, stderr, artifact 경로와 image dimensions을 검증 evidence로 유지한다.
- Official Mermaid CLI path는 semantic layout과 neutral light/dark theme를 사용한다. 최종 Go renderer만 120-cell viewport와 Flow auto-fit을 사용하며, 요청 방향이 120 cells를 넘으면 반대 방향을 시도한다.
- Mermaid source는 사용자가 재사용을 요청했을 때만 함께 보여준다.
- 이미지 변환기나 image attachment 도구가 없는 surface에서는 동일 source를 `scripts/render.sh`로 렌더하고 성공 stdout을 `text` code fence에 그대로 넣는다.
- 두 방향 모두 viewport를 넘거나 route가 모호하면 label·node 수를 줄이거나 핵심 이야기를 두 도식으로 나눠 한 번만 재시도한다.
- 두 번째 실패 시 수동 Unicode 도식을 만들지 않는다. 도식을 생략하고 확인된 사실만 text로 설명하며 renderer 한계를 한 문장으로 밝힌다.
- MCP server와 widget은 외부 network를 사용하지 않으며 runtime package download를 수행하지 않는다. Official Mermaid CLI는 별도 설치 단계에서만 exact lockfile로 설치하고 render 중에는 다운로드·업데이트하지 않는다. Artifact renderer는 권한 제한 temp file만 만들며 project file을 변경하지 않는다.
