package workflow

import (
	"testing"
)

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    TokenState
		to      TokenState
		wantErr bool
	}{
		{"pending to prepared", StatePending, StatePrepared, false},
		{"pending to expired", StatePending, StateExpired, false},
		{"pending to executing", StatePending, StateExecuting, true},
		{"pending to completed", StatePending, StateCompleted, true},
		{"pending to failed", StatePending, StateFailed, true},
		{"prepared to executing", StatePrepared, StateExecuting, false},
		{"prepared to expired", StatePrepared, StateExpired, false},
		{"prepared to completed", StatePrepared, StateCompleted, true},
		{"executing to completed", StateExecuting, StateCompleted, false},
		{"executing to failed", StateExecuting, StateFailed, false},
		{"completed to anything", StateCompleted, StatePending, true},
		{"failed to anything", StateFailed, StatePending, true},
		{"expired to anything", StateExpired, StatePending, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransition(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTransition(%s, %s) error = %v, wantErr %v", tt.from, tt.to, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePayload(t *testing.T) {
	tests := []struct {
		name       string
		actionType string
		payload    map[string]interface{}
		wantErr    bool
	}{
		{
			"valid tool_call",
			ActionTypeToolCall,
			map[string]interface{}{
				"tool_name": "create_issue",
				"args":      map[string]interface{}{"title": "test"},
				"actor_id":  "agent-1",
			},
			false,
		},
		{
			"missing tool_name",
			ActionTypeToolCall,
			map[string]interface{}{
				"args":     map[string]interface{}{},
				"actor_id": "agent-1",
			},
			true,
		},
		{
			"missing actor_id",
			ActionTypeToolCall,
			map[string]interface{}{
				"tool_name": "create_issue",
				"args":      map[string]interface{}{},
			},
			true,
		},
		{
			"unknown action type",
			"unknown_action",
			map[string]interface{}{},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePayload(tt.actionType, tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePayload(%s, %v) error = %v, wantErr %v", tt.actionType, tt.payload, err, tt.wantErr)
			}
		})
	}
}
