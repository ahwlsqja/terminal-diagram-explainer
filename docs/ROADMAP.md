# 확장 로드맵

확장은 기능 수보다 결정성·자원 상한·terminal safety를 먼저 고정합니다.

## 1. Cycle — 완료 (0.3.0)

- Tarjan SCC membership과 source-edge-order greedy feedback 분류를 사용합니다.
- LR bottom gutter, TD right gutter, self-loop, cycle+tail, disconnected cycle을 지원합니다.
- 32,768 work-step budget과 48 nodes/96 edges 단일 SCC fixture를 검증합니다.
- Feedback label은 inline이 아니라 bounded `feedback:` legend에 표시합니다.
- Skip-rank forward edge도 outer route를 사용하고 label은 `routed:` legend에 표시합니다.

## 2. Subgraph

- forest 형태의 scope model을 사용하고 graph-global node ID를 유지합니다.
- `MaxSubgraphs=32`, `MaxSubgraphDepth=8`로 시작합니다.
- malformed `end`, duplicate membership, cross-subgraph edge, nested CJK label을 property/golden test로 고정합니다.

## 3. Sequence Diagram

- 초기 상한: participants 16, messages 96, fragments 32, nesting 8.
- request/return, fan-out, self-message, Unicode/ASCII golden을 먼저 추가합니다.
- fragment/activation은 기본 message model의 결정성·allocation gate 통과 후 추가합니다.

## 4. ER Diagram

- 초기 상한: entities 32, relationships 64, attributes total 192, entity당 32.
- cardinality, PK/FK, self-relation, disconnected component, CJK label을 먼저 검증합니다.

## 공통 완료 게이트

- parser/renderer fuzz, race, vet, offline build
- 동일 입력 256회 byte-identical output
- canvas 512×512 hard cap과 clipping 없는 bounds error
- 최대 fixture 2,500 allocations/run 이하
- 입력 4배 증가 시 allocations 4배 이하
- ASCII drawing glyph-only, label codepoint 보존, trailing whitespace 없음
