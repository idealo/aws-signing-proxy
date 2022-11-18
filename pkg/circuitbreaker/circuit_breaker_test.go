package circuitbreaker

import (
	"errors"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCircuitBreakerOpenState(t *testing.T) {

	breaker := NewCircuitBreaker()

	for i := 0; i < 10; i++ {
		breaker.Execute(func() (interface{}, error) {
			return nil, errors.New("something went wrong")
		})
	}

	assert.Equal(t, gobreaker.StateOpen, breaker.breaker.State())

	expectedStateMetric := `
# HELP auth_circuit_breaker_state State of the authorization circuit breaker
# TYPE auth_circuit_breaker_state gauge
auth_circuit_breaker_state{state="closed"} 0
auth_circuit_breaker_state{state="half-open"} 0
auth_circuit_breaker_state{state="open"} 1
`

	if err := testutil.CollectAndCompare(cbStateGauge, strings.NewReader(expectedStateMetric), "auth_circuit_breaker_state"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}

	expectedCountMetric := `
# HELP auth_circuit_breaker_count Circuit breaker request count
# TYPE auth_circuit_breaker_count gauge
auth_circuit_breaker_count{type="consecutive_failures"} 0
auth_circuit_breaker_count{type="consecutive_successes"} 0
auth_circuit_breaker_count{type="requests"} 0
auth_circuit_breaker_count{type="total_failures"} 0
auth_circuit_breaker_count{type="total_successes"} 0
`

	if err := testutil.CollectAndCompare(cbCounterGauge, strings.NewReader(expectedCountMetric), "auth_circuit_breaker_count"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestCircuitBreakerClosedState(t *testing.T) {

	breaker := NewCircuitBreaker()

	for i := 0; i < 10; i++ {
		breaker.Execute(func() (interface{}, error) {
			return "okay", nil
		})
	}

	assert.Equal(t, gobreaker.StateClosed, breaker.breaker.State())

	expected := `
# HELP auth_circuit_breaker_state State of the authorization circuit breaker
# TYPE auth_circuit_breaker_state gauge
auth_circuit_breaker_state{state="closed"} 1
auth_circuit_breaker_state{state="half-open"} 0
auth_circuit_breaker_state{state="open"} 0
`

	if err := testutil.CollectAndCompare(cbStateGauge, strings.NewReader(expected), "auth_circuit_breaker_state"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}

	expectedCountMetric := `
# HELP auth_circuit_breaker_count Circuit breaker request count
# TYPE auth_circuit_breaker_count gauge
auth_circuit_breaker_count{type="consecutive_failures"} 0
auth_circuit_breaker_count{type="consecutive_successes"} 10
auth_circuit_breaker_count{type="requests"} 10
auth_circuit_breaker_count{type="total_failures"} 0
auth_circuit_breaker_count{type="total_successes"} 10
`

	if err := testutil.CollectAndCompare(cbCounterGauge, strings.NewReader(expectedCountMetric), "auth_circuit_breaker_count"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestCircuitBreakerFailureThresholdConfigParsing(t *testing.T) {

	os.Setenv("ASP_CIRCUIT_BREAKER_FAILURE_THRESHOLD", "50")

	breaker := NewCircuitBreaker()

	for i := 0; i < 10; i++ {
		breaker.Execute(func() (interface{}, error) {
			return nil, errors.New("something went wrong")
		})
	}

	assert.Equal(t, gobreaker.StateClosed, breaker.breaker.State())

	defer t.Cleanup(func() {
		os.Setenv("ASP_CIRCUIT_BREAKER_FAILURE_THRESHOLD", "")
	})

}

func TestCircuitBreakerTimeoutConfigParsing(t *testing.T) {

	os.Setenv("ASP_CIRCUIT_BREAKER_TIMEOUT", "300ms")

	breaker := NewCircuitBreaker()

	for i := 0; i < 10; i++ {
		breaker.Execute(func() (interface{}, error) {
			return nil, errors.New("something went wrong")
		})
	}

	time.Sleep(310 * time.Millisecond)
	assert.Equal(t, gobreaker.StateHalfOpen, breaker.breaker.State())

	defer t.Cleanup(func() {
		os.Setenv("ASP_CIRCUIT_BREAKER_TIMEOUT", "")
	})

}
