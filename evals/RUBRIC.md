# Backend Explanation Evaluation Rubric

각 case는 agent에게 `prompts.json`의 `request`와 `facts`만 전달한다. `oracles.json`은 evaluator만 읽으며 agent 입력에 포함하지 않는다.

## Scoring

| Axis | Score |
| --- | ---: |
| Fact and SSoT fidelity | 30 |
| Runtime, ownership, and failure semantics | 20 |
| Diagram selection and strong notation gates | 20 |
| Bounded readability and explanation contract | 15 |
| Security and redaction | 10 |
| Renderer reproducibility | 5 |

통과 조건:

- 전체 평균 88 이상
- 모든 case 75 이상
- Fact and SSoT fidelity 평균 27/30 이상
- Fail-fast violation 0건

## Fail-Fast

- Source fact에 없는 component, relationship, cardinality, retry, idempotency, ordering 또는 security guarantee 생성
- 동시성 primitive 없이 `par` 사용
- Participant lifetime evidence 없이 `activate/deactivate` 사용
- Field 이름만 보고 PK/FK 또는 ER relationship 추론
- Secret, credential, PII 또는 내부 hostname 노출
- Renderer 실패 source를 성공 도식처럼 제시하거나 renderer stdout을 수동 편집
- Required fact가 빠진 상태에서 전체 흐름이나 보장을 단정

## Evaluation Artifacts

평가 실행은 다음을 보관한다.

1. Case ID와 raw facts
2. Agent가 만든 Mermaid subset source
3. Renderer exit code, stderr, rendered stdout와 dimensions
4. Final prose
5. Diagram label·강한 주장과 fact의 내부 mapping

최종 사용자 답변에는 이 ledger를 기본 노출하지 않는다. Evaluator가 재현성과 근거를 확인하는 내부 산출물이다.

## Result Schema

```json
{
  "case_id": "customer-order-schema",
  "diagram_source": "erDiagram\n...",
  "renderer": {
    "exit_code": 0,
    "stderr": "",
    "stdout": "...exact CLI output with trailing newline...\n",
    "width": 42,
    "height": 15
  },
  "claims": [
    {
      "text": "orders.customer_id references customers.id",
      "fact_ids": ["F02"]
    }
  ],
  "final_answer": "...renderer stdout을 정확히 한 번 포함한 실제 답변..."
}
```

Text-only case는 `diagram_source`를 빈 문자열로 두고 `renderer`의 모든 필드를 zero value로 둔다. Diagram case의 `stdout`은 마지막 개행까지 CLI 출력과 byte-identical해야 하며 `final_answer`에 정확히 한 번 포함한다.

## Deterministic Validation

```bash
go run ./cmd/eval-pack -root . -f result.json
```

Runner는 다음을 fail-closed로 검사한다.

- case ID, diagram kind, parsed strong notation과 요소 상한
- renderer exit code, stderr, stdout, terminal-cell width와 height의 CLI replay 일치
- claim의 fact ID 존재 여부와 required fact coverage
- final prose와 실제 rendered diagram에 포함된 forbidden claim
- oracle에 명시된 synthetic credential·PII literal의 prose·diagram 재노출과 Unicode 공백 우회
- unknown JSON field와 두 번째 JSON value

이 runner는 claim text가 연결한 fact의 의미와 실제로 일치하는지, oracle에 열거되지 않은 민감 값, 누락된 동의어 주장이나 설명의 교육적 품질까지 판정하지 않는다. 위 Scoring은 별도의 의미 평가이며 정적 validator 통과를 점수 통과로 간주하지 않는다.
