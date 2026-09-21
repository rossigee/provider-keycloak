package events

import (
	"context"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockEventsClient struct {
	*testhelpers.BaseMockClient
	getEventFn    func(ctx context.Context, realm, id string) (interface{}, error)
	updateEventFn func(ctx context.Context, realm string, event interface{}) error
}

func (m *mockEventsClient) GetEvent(ctx context.Context, realm, id string) (interface{}, error) {
	if m.getEventFn != nil {
		return m.getEventFn(ctx, realm, id)
	}
	return nil, nil
}

func TestEventsObserveExists(t *testing.T) {
	getCalled := false
	mockClient := &mockEventsClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getEventFn: func(ctx context.Context, realm, id string) (interface{}, error) {
			getCalled = true
			return map[string]string{"id": id, "type": "LOGIN"}, nil
		},
	}

	event, err := mockClient.GetEvent(context.Background(), "test-realm", "event-123")
	if err != nil {
		t.Fatalf("GetEvent failed: %v", err)
	}
	if !getCalled {
		t.Fatal("GetEvent was not called")
	}
	if event == nil {
		t.Fatal("Expected event but got nil")
	}
}

func (m *mockEventsClient) UpdateEvent(ctx context.Context, realm string, event interface{}) error {
	if m.updateEventFn != nil {
		return m.updateEventFn(ctx, realm, event)
	}
	return nil
}

func TestEventsUpdateSuccess(t *testing.T) {
	updateCalled := false
	mockClient := &mockEventsClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		updateEventFn: func(ctx context.Context, realm string, event interface{}) error {
			updateCalled = true
			return nil
		},
	}

	err := mockClient.UpdateEvent(context.Background(), "test-realm", map[string]string{"type": "LOGIN"})
	if err != nil {
		t.Fatalf("UpdateEvent failed: %v", err)
	}
	if !updateCalled {
		t.Fatal("UpdateEvent was not called")
	}
}
