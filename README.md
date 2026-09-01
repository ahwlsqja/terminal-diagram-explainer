# Terminal Diagram Explainer

복잡한 소프트웨어 아키텍처·데이터·API·Worker 흐름을 interactive Mermaid UI와 bounded fallback으로 설명하는 Codex 플러그인입니다.

구성은 세 부분으로 나뉩니다.

- `render_diagram`: Mermaid가 bundled된 sandboxed MCP App에서 semantic SVG를 만들고 pan·zoom·fit·source 전환 UI로 보여주는 기본 경로
- `term-diagram`: 외부 Go 모듈, 네트워크, subprocess, CGO가 없는 bounded Flowchart·Sequence·ER·State → Unicode/ASCII/SVG renderer
- `terminal-diagram-explainer`: 시각화 요청을 MCP UI로 보내고 UI가 없는 surface에서 PNG·HTML·terminal 순으로 fallback하는 Codex Skill

플러그인은 표현 방식만 추가하며 프로젝트의 `AGENTS.md`, SDLC, workflow 또는 repo-local Skill을 변경하지 않습니다.

## Graphical UI

Codex Desktop의 지원 surface에서는 Skill이 표준 Mermaid source를 `render_diagram` MCP tool에 전달합니다. Tool은 외부 network 없이 bundled Mermaid로 SVG를 만들며, source 확인과 pan·zoom·fit을 한 화면에서 제공합니다. UI를 표시하지 않는 client에서도 tool 결과가 text와 structured data로 남고 기존 artifact path가 fallback합니다.

Graphical source는 standard Mermaid 11을 사용하되 Skill은 software explanation에 필요한 Flowchart·Sequence·ER·State로 제한합니다. `click`, remote image/icon, CSS URL/import, init directive, active HTML은 server와 widget 양쪽에서 거부합니다.

## Terminal fallback 문법

```text
flowchart LR
subgraph Service[Service boundary]
Receive[Request] --> Validate{Valid?}
end
subgraph Data[Data boundary]
Store[(Canonical model)]
end
Validate -->|yes| Store
Validate -.->|no| Reject[Reject + observe]
```

- 방향: `LR`, `TD`, `TB`
- 노드: `ID`, `ID[label]`, `ID{decision}`, `ID[(data store)]`
- edge: `-->`, `-.->`, 선택적 `|label|`
- 한 줄 chain, `%%` 주석
- subgraph: `subgraph ID`, `subgraph ID[label]`, 중첩 `end`
- node ID는 전체 graph에서 유일하며 각 node는 root 또는 하나의 subgraph에 직접 소속됩니다.
- cycle과 self-loop는 SCC 분석 후 외곽 feedback route로 렌더링하며 label은 `feedback:` legend에 표시합니다.
- 중간 rank를 건너뛰거나 혼합 fan-out/fan-in junction을 만드는 edge는 node 관통을 피하도록 외곽 route를 사용하며, label 유무와 무관하게 endpoint를 `routed:` manifest에 표시합니다.
- cross-subgraph edge는 endpoint의 최소 공통 조상 frame 안에서만 우회하며, label 유무와 무관하게 endpoint를 `routed:` manifest에 표시합니다.
- 인접 rank는 bounded median sweep으로 crossing을 줄이고 edge별 lane을 예약합니다. 안전하게 분리할 수 있는 혼합 junction은 outer route와 manifest로 승격하고, 동일 endpoint의 parallel edge처럼 의미를 보존할 수 없는 경우는 오류를 반환합니다.
- Canvas는 N/E/S/W 연결 방향으로 corner·tee·junction glyph를 합성하며 동일 endpoint의 parallel forward edge는 조용히 축약하지 않고 거부합니다.

```text
sequenceDiagram
participant Client as Browser Client
participant API as API Gateway
participant Worker as Async Worker
Client ->> API: POST /events
API ->> Worker: enqueue
API -->> Client: 202 Accepted
Worker ->> Worker: record metrics
```

- participant: `participant ID`, `participant ID as Label`
- message: request `->>`, return `-->>`, 첫 `:` 뒤의 필수 label
- participant는 message보다 먼저 명시적으로 선언하며 source order가 lifeline 순서입니다.
- fan-out은 같은 sender의 연속 message, self-message는 같은 ID endpoint로 표현합니다.
- 최대 16 participants, 96 messages입니다.
- `loop label ... end`, `alt label ... else label ... end`, `opt label ... end`를 지원합니다.
- 모든 fragment branch는 message를 포함해야 하며 최대 32 fragments, 중첩 깊이 8, 전체 256 steps입니다.
- `activate ID`와 `deactivate ID`는 participant별 LIFO active interval을 표시합니다. 최대 96 activations, participant별 depth 8입니다.
- Activation pair 사이에는 message가 있어야 하고 fragment 시작·`else`·`end` 경계를 넘을 수 없습니다.
- `par label ... and label ... end`는 독립 branch를 source/display order로 보여줍니다. 이 세로 순서는 실제 동시 실행 순서나 happens-before를 뜻하지 않습니다.
- `par`는 최소 두 개의 nonempty branch를 요구하며 activation은 각 branch 안에서 완결되어야 합니다.

```text
erDiagram
Customer ||--o{ Order : places
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
```

- Entity block은 `ID { ... }` 또는 `ID[display label] { ... }`입니다.
- Attribute는 `type name [PK] [FK] [UNIQUE] [NOT NULL]`이며 marker 조합과 순서 변경을 허용합니다. 출력은 `PK FK UNIQUE NOT NULL type name` 순서로 정규화합니다.
- PK가 NOT NULL을 암묵 생성하지 않으며, relationship·constraint·target을 field 이름에서 추론하지 않습니다.
- Multiline entity block은 2~8 columns의 `PRIMARY KEY (...)`, `UNIQUE (...)`, `FOREIGN KEY (...) REFERENCES Entity(...)` table constraint를 지원합니다.
- Composite FK는 ordered column mapping만 표시하며 relationship·cardinality·attribute FK marker를 자동 생성하지 않습니다.
- Cardinality marker는 `0..1`, `1`, `0..N`, `1..N` 네 종류를 명시합니다.
- Self·duplicate relationship과 disconnected entity를 지원합니다.
- 최대 32 entities, 64 relationships, attributes 총 192/entity당 32입니다.
- Named `CONSTRAINT`, `DEFAULT`, `CHECK`, referential actions, inline table constraint, `classDef`, `style`, `click`, HTML/Markdown label, Sequence/ER note와 advanced ER semantics는 아직 명시적으로 거부합니다.

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

- Header는 exact `stateDiagram-v2`이며 optional `direction TD|LR`은 첫 declaration/transition 전에 한 번만 사용합니다.
- State는 `state ID` 또는 `state "display label" as ID`로 명시 선언하며 transition endpoint가 state를 자동 생성하지 않습니다.
- Concrete transition은 `A --> B`, `A --> B : event`, `A --> B : event [guard]`입니다. Event와 guard를 분리해 보존합니다.
- Initial은 정확히 하나의 `[*] --> State`, final은 0개 이상의 `State --> [*]`이며 pseudo transition에는 label을 붙이지 않습니다.
- Choice는 `state ID <<choice>>` 또는 `state "display label" as ID <<choice>>`로만 명시합니다. Ordinary state에서 정확히 하나의 inbound, 서로 다른 ordinary target으로 2~8개의 `: [guard]` outbound가 필요합니다.
- Choice guard는 source text를 보존하는 opaque 조건입니다. 상호배타성·우선순위·default·완전성을 계산하거나 추론하지 않으며 choice incident transition에는 policy를 붙이지 않습니다.
- 직접 확인된 transition policy는 `policy <exact labeled transition> :: <retry|timeout|compensation> "detail"`로 별도 선언합니다. Policy는 기존 transition을 EOF에서 정확히 참조하며 state나 edge를 만들지 않습니다.
- Policy detail은 source contract의 원문만 보존합니다. Retry 횟수·backoff, timeout 기준, compensation action·성공·원자성·idempotency를 자동 추론하지 않습니다.
- 최대 32 states, 64 transitions, 64 policies, ID 64 bytes, state/transition/policy detail 96 cells입니다.
- Self/cycle은 bounded connector와 `feedback:` legend로 표시하며 disconnected state도 source order로 보존합니다.
- Composite/nested state, fork/join/history, note/style과 이름 기반 choice/policy 승격은 지원하지 않습니다. Transition label의 policy-like text는 ordinary event로만 보존하며 policy가 되지 않습니다.

## 개발 검증

```bash
GOTOOLCHAIN=local GOPROXY=off go test ./...
GOTOOLCHAIN=local GOPROXY=off go test -race ./...
GOTOOLCHAIN=local GOPROXY=off go vet ./...
GOTOOLCHAIN=local GOPROXY=off go list -m all
cd plugins/terminal-diagram-explainer/mcp
npm ci --ignore-scripts
npm audit --omit=dev
npm test
npm run test:visual
```

Standalone CLI 기본 viewport는 240×200입니다. 좁은 출력 surface에서는 폭과 자동 방향 전환을 명시할 수 있습니다.

```bash
printf '%s\n' "$diagram_source" | term-diagram -width 120 -height 200 -fit
printf '%s\n' "$diagram_source" | term-diagram -format svg -width 120 -height 200 -fit > diagram.svg
printf '%s\n' "$diagram_source" | term-diagram -format html -width 120 -height 200 -fit > diagram.html
```

Codex plugin wrapper는 코드 블록 soft-wrap과 font line-height 단절을 피하기 위해 120×200 Flow auto-fit SVG를 만들고, `sips`(macOS)·`rsvg-convert`·ImageMagick 중 설치된 로컬 변환기로 PNG를 생성합니다. 같은 geometry를 pan·zoom·fit 가능한 self-contained HTML로도 제공하며 네트워크 다운로드는 수행하지 않습니다.

`evals/prompts.json`에는 agent에게 전달할 backend/core 설명 18개가 있고, `evals/oracles.json`에는 평가할 때만 읽는 기준이 분리되어 있습니다. Reference diagram은 실제 parser/renderer로 재생하며 strong notation evidence gate, text-only 선택, SSoT·ordering·security·redaction case를 검증합니다.

Agent 결과 JSON은 다음 명령으로 정적 fail-fast 검증을 실행합니다.

```bash
go run ./cmd/eval-pack -root . -f result.json
```

이 검증은 diagram kind·표기·요소 상한, fact ID coverage, 금지 주장, renderer exit/stderr/stdout/dimensions, 최종 답변의 stdout 원문 포함을 확인합니다. Claim 문장이 연결한 fact의 의미와 실제로 일치하는지는 자동 판정하지 않으며 [evals/RUBRIC.md](evals/RUBRIC.md)의 의미 평가로 별도 확인합니다.

18개 전체를 1~3회 반복 평가할 때는 agent artifact와 evaluator review를 분리해 batch gate를 실행합니다.

```bash
go run ./cmd/eval-pack -root . -corpus-digest
go run ./cmd/eval-pack -root . -inspect-batch submission.json > binding.json
go run ./cmd/eval-pack -root . -batch submission.json -review review.json > report.json
```

각 run은 18개 case를 정확히 한 번 포함해야 합니다. Runner는 static validation을 통과한 case에 renderer reproducibility 5점을 직접 부여하고, run별 평균 88·모든 case 75·Fact/SSoT 평균 27·fail-fast 0건을 독립적으로 판정합니다. 반복 결과의 exact score variance와 canonical artifact SHA-256 distinct count는 정보성 지표로 report합니다.

## 설치

Go 1.25 이상, Node.js 20 이상, Codex CLI가 필요합니다. Graphical MCP runtime은 plugin에 build artifact로 포함되고 fallback renderer는 `$CODEX_HOME/bin/term-diagram`에 설치됩니다.

```bash
scripts/install-local.sh
scripts/install-global-guidance.sh
codex plugin marketplace add ahwlsqja/terminal-diagram-explainer --ref main
codex plugin add terminal-diagram-explainer@terminal-diagrams
```

전역 기본 설명 선호가 필요하지 않으면 `scripts/install-global-guidance.sh`는 생략할 수 있습니다. 전역 `AGENTS.md` 전체 교체 후에는 이 스크립트만 다시 실행합니다.

자세한 입력·출력 경계는 [SECURITY.md](SECURITY.md), 문법은 [docs/GRAMMAR.md](docs/GRAMMAR.md), 확장 계획은 [docs/ROADMAP.md](docs/ROADMAP.md)를 참고합니다.
