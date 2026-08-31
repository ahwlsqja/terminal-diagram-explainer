# 확장 로드맵

확장은 기능 수보다 결정성·자원 상한·terminal safety를 먼저 고정합니다.

## 1. Cycle — 완료 (0.3.0)

- Tarjan SCC membership과 source-edge-order greedy feedback 분류를 사용합니다.
- LR bottom gutter, TD right gutter, self-loop, cycle+tail, disconnected cycle을 지원합니다.
- 32,768 work-step budget과 48 nodes/96 edges 단일 SCC fixture를 검증합니다.
- Feedback label은 inline이 아니라 bounded `feedback:` legend에 표시합니다.
- Skip-rank forward edge도 outer route를 사용하고 label은 `routed:` legend에 표시합니다.

## 2. Subgraph — 완료 (0.4.0)

- `Node.Scope` 단일 membership과 source-order parent forest를 사용하고 graph-global node ID를 유지합니다.
- `MaxSubgraphs=32`, `MaxSubgraphDepth=8`, `ScopeRef` representability guard를 적용합니다.
- LR y-band, TD x-band, 중첩 frame, child-only parent를 지원합니다.
- Cross-scope·feedback·skip-rank edge는 frame-safe corridor와 방향별 portal을 사용합니다.
- malformed `end`, duplicate membership, cross-subgraph edge, nested CJK label, long TD inline label을 parser/property/golden test로 고정합니다.

## 3. Sequence Diagram — 완료 (0.5.0)

- 독립 participant/message 모델과 앱 header dispatcher로 Flow 경계를 유지합니다.
- participants 16, messages 96, label 96 cells 상한을 parser와 renderer 양쪽에서 검증합니다.
- request/return, fan-out, self-message, 양방향 arrow, Unicode/ASCII를 지원합니다.
- 96개 일반 message는 2-row pitch로 기본 200행 canvas에 들어가며 self-message 혼합은 exact bounds를 넘으면 fail-closed 처리합니다.
- 이 단계에서는 fragment와 activation을 기본 message model과 분리했고, structured fragment는 다음 0.6 단계에서 추가했습니다.

## 4. Structured Sequence Fragments — 완료 (0.6.0)

- Message-only legacy AST와 ordered `Steps` extended AST를 nil 기준 상호 배타 모드로 유지합니다.
- `loop`, `alt/else`, `opt`와 nested frame을 지원합니다.
- fragments 32, depth 8 hard limit을 parser와 renderer 양쪽에서 검증합니다.
- Empty branch, unmatched/duplicate branch, malformed `end`, invalid direct Step variant를 fail-closed 처리합니다.
- Explicit activation은 fragment branch state 계약을 먼저 고정한 0.6.1 확장으로 남깁니다.
- `par/and`는 source-order가 실행 순서처럼 오해되지 않는 비시간 branch 표현을 설계한 뒤 0.7에서 검토합니다.

## 5. Explicit Sequence Activation — 완료 (0.6.1)

- `activate`/`deactivate`를 participant별 LIFO interval로 렌더링합니다.
- 총 activations 96, participant depth 8, timeline steps 256 hard limit을 parser와 renderer 양쪽에서 검증합니다.
- Zero-message, unmatched, unclosed activation을 거부합니다.
- Activation은 fragment 경계를 넘지 못하며 branch 내부에서 완결되어야 합니다.
- Active endpoint·nested bar·self rail은 actual attachment 좌표로 재계산합니다.

## 6. ER Diagram

- 초기 상한: entities 32, relationships 64, attributes total 192, entity당 32.
- cardinality, PK/FK, self-relation, disconnected component, CJK label을 먼저 검증합니다.

## 공통 완료 게이트

- parser/renderer fuzz, race, vet, offline build
- 동일 입력 256회 byte-identical output
- canvas 512×512 hard cap과 clipping 없는 bounds error
- 최대 fixture 2,500 allocations/run 이하
- 입력 4배 증가 시 allocations 4배 이하
- ASCII drawing glyph-only, label codepoint 보존, trailing whitespace 없음
