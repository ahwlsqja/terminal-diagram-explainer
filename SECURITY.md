# 입력·실행 경계

이 도구는 Codex가 생성한 작은 설명용 Flowchart만 처리합니다. 임의 Mermaid 호환 renderer가 아닙니다.

## 불변식

- 입력은 256 KiB 이하입니다.
- 최대 2,048 lines, 48 nodes, 96 edges, label 96 cells입니다.
- canvas는 최대 240×200 cells이며 clipping 대신 오류를 반환합니다.
- invalid/unsupported syntax는 line/column이 있는 오류로 fail-closed 처리합니다.
- 입력 검증·파싱·렌더 실패는 stdout에 아무것도 기록하지 않습니다. OS·pipe·writer가 실제 쓰기 도중 실패한 경우에는 이미 전달된 byte를 회수할 수 없으므로 exit code 1과 stderr 진단으로 알립니다.
- label의 terminal control·bidi·format 문자는 렌더 전에 거부합니다.
- 네트워크, HTTP server, shell, subprocess, environment-driven behavior, runtime file write가 없습니다.
- `CGO_ENABLED=0`, `GOPROXY=off`에서 build/test할 수 있으며 `go list -m all`은 자기 모듈 하나만 출력해야 합니다.
- map은 lookup에만 사용하고 배치·출력 순서는 source-order slice로 결정합니다.

## Upstream audit에서 확인한 제거 대상

평가 대상 `AlexanderGrooff/mermaid-ascii` v1.5.0에서는 다음이 기본 stdin 또는 선택적 web 경로에서 확인됐습니다.

- malformed `classDef`의 index panic
- label ESC byte의 terminal 출력 통과
- LR chain의 edge별 full-canvas 보관으로 인한 quadratic peak memory 증가
- 선택적 web server, external `git` 실행, mutable global state

맞춤 구현은 해당 코드를 전용하지 않고 지원 문법과 실행 표면을 축소해 위 경로를 구조적으로 제거했습니다.

## Provenance

- 기능 요구는 Mermaid의 공개 텍스트 문법 subset과 일반 개발자 설명 요구에서 정의했습니다.
- upstream 구현을 검토한 동일 주체가 작성했으므로 법적 의미의 clean-room이라고 표현하지 않습니다.
- upstream source code, 함수 구조, golden fixture 또는 test corpus를 복사하지 않은 독립 재작성입니다.
