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
- Field·index 이름, type, 관례만 보고 PK/FK/UNIQUE/NOT NULL, ordered composite mapping 또는 ER relationship 추론
- Enum·status·함수 이름만 보고 state transition, initial/final, guard 또는 terminal lifecycle 추론
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

## Batch Evaluation

Batch v1은 agent가 만든 artifact와 evaluator가 작성한 의미 점수를 서로 다른 파일로 받는다. Agent submission에 점수나 pass 판정을 넣지 않는다.

Submission의 축약 형태:

```json
{
  "schema": "eval-pack.batch.v1",
  "subject_id": "terminal-diagram-explainer-0.13.0",
  "corpus_digest": "<64 lowercase hex>",
  "runs": [
    {
      "run_id": "run-01",
      "results": ["18 EvaluationResult objects"]
    }
  ]
}
```

Review의 축약 형태:

```json
{
  "schema": "eval-pack.review.v1",
  "evaluator_id": "independent-evaluator-01",
  "subject_id": "terminal-diagram-explainer-0.13.0",
  "corpus_digest": "<binding corpus_digest>",
  "submission_digest": "<binding submission_digest>",
  "reviews": [
    {
      "run_id": "run-01",
      "case_id": "customer-order-schema",
      "fact_ssot_fidelity": 30,
      "runtime_ownership_failure": 20,
      "diagram_notation": 20,
      "readability_contract": 15,
      "security_redaction": 10,
      "semantic_fail_fast": []
    }
  ]
}
```

Evaluator는 먼저 현재 corpus digest와 submission binding을 만든다.

```bash
go run ./cmd/eval-pack -root . -corpus-digest
go run ./cmd/eval-pack -root . -inspect-batch submission.json > binding.json
```

`binding.json`은 raw answer를 복제하지 않고 `(run_id, case_id, result_digest)`와 corpus·submission digest, renderer version만 포함한다. Review의 `subject_id`, `corpus_digest`, `submission_digest`는 이 binding과 정확히 같아야 한다.

완성된 review를 집계한다.

```bash
go run ./cmd/eval-pack -root . -batch submission.json -review review.json > report.json
```

### Batch Gates

- Schema `eval-pack.batch.v1`은 1~3 runs와 run당 현재 18 cases를 정확히 한 번 요구한다.
- Evaluator는 30/20/20/15/10의 의미 축만 정수로 입력한다. Static validation 성공 시 runner가 renderer reproducibility 5점을 부여한다.
- Static validation에 실패한 case의 evaluator 점수는 집계하지 않고 해당 관측치를 0점으로 처리한다.
- 각 run이 독립적으로 전체 평균 88 이상, 모든 case 75 이상, Fact/SSoT 평균 27 이상, static·semantic fail-fast 0건을 만족해야 한다.
- 다른 run의 높은 점수로 실패 run을 상쇄하지 않는다.
- 2~3 runs이면 case별 population variance를 exact numerator/denominator로 기록한다. Canonical artifact digest의 distinct count는 정보성 지표이며 동일 결과 자체를 실패로 취급하지 않는다.
- Semantic fail-fast는 rubric 항목에 대응하는 고정 code와 `fact:F02`, `claim[1]` 형태의 안전한 evidence reference를 하나 이상 가진다.
- Report에는 raw validation error, answer, secret 또는 PII를 복제하지 않고 safe code만 남긴다.
- `subject_id`, `evaluator_id`, `run_id`는 report에 표시되는 공개·비민감 metadata만 사용한다. Structural-invalid report는 입력 식별자를 반사하지 않는다.

### Input Boundaries

- Submission 64 MiB, review 1 MiB, canonical result 1 MiB, JSON depth 64
- Diagram source 256 KiB, renderer stdout 200 KiB, stderr 16 KiB, final answer 256 KiB
- Claims 256개, 전체 claim text 64 KiB, claim당 fact IDs 64개
- Unknown field, trailing JSON value, 모든 object depth의 duplicate key, 누락·`null`·소수점 score를 거부한다.

Digest binding은 review가 현재 corpus와 정확한 submission을 대상으로 했는지 확인한다. `evaluator_id`는 감사용 metadata이며 신원을 인증하지 않는다. Runner는 독립 모델 호출 여부, evaluator 독립성 또는 실행 freshness를 증명하지 않으므로 release gate에 사용할 때는 실행 환경이 이 provenance를 별도로 보장해야 한다.
