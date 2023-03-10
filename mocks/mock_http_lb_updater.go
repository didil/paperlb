// Code generated by MockGen. DO NOT EDIT.
// Source: services/http_lb_updater.go

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	services "github.com/didil/paperlb/services"
	gomock "github.com/golang/mock/gomock"
)

// MockHTTPLBUpdaterClient is a mock of HTTPLBUpdaterClient interface.
type MockHTTPLBUpdaterClient struct {
	ctrl     *gomock.Controller
	recorder *MockHTTPLBUpdaterClientMockRecorder
}

// MockHTTPLBUpdaterClientMockRecorder is the mock recorder for MockHTTPLBUpdaterClient.
type MockHTTPLBUpdaterClientMockRecorder struct {
	mock *MockHTTPLBUpdaterClient
}

// NewMockHTTPLBUpdaterClient creates a new mock instance.
func NewMockHTTPLBUpdaterClient(ctrl *gomock.Controller) *MockHTTPLBUpdaterClient {
	mock := &MockHTTPLBUpdaterClient{ctrl: ctrl}
	mock.recorder = &MockHTTPLBUpdaterClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHTTPLBUpdaterClient) EXPECT() *MockHTTPLBUpdaterClientMockRecorder {
	return m.recorder
}

// Delete mocks base method.
func (m *MockHTTPLBUpdaterClient) Delete(ctx context.Context, url string, params *services.DeleteParams) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", ctx, url, params)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockHTTPLBUpdaterClientMockRecorder) Delete(ctx, url, params interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockHTTPLBUpdaterClient)(nil).Delete), ctx, url, params)
}

// Update mocks base method.
func (m *MockHTTPLBUpdaterClient) Update(ctx context.Context, url string, params *services.UpdateParams) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", ctx, url, params)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update.
func (mr *MockHTTPLBUpdaterClientMockRecorder) Update(ctx, url, params interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockHTTPLBUpdaterClient)(nil).Update), ctx, url, params)
}
