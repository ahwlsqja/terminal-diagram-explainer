package app

import (
	"strings"
	"testing"
)

func TestRunSequenceParRendersWithoutClaimingExecutionOrder(t *testing.T) {
	source := `sequenceDiagram
participant API
participant Email
participant SMS
par notify
API ->> Email: email
and sms
API ->> SMS: sms
end`
	result := invoke(nil, strings.NewReader(source))
	if result.code != 0 || result.stderr != "" || !strings.Contains(result.stdout, "par (display order only): notify") || !strings.Contains(result.stdout, "and: sms") {
		t.Fatalf("result=%+v", result)
	}
}
