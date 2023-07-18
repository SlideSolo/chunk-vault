package main

import "github.com/stretchr/testify/mock"

type MetricsMock struct {
	mock.Mock
}

func (m *MetricsMock) IncApiRequestsCount(method, endpoint, status string) {} //nolint
