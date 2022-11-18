package circuitbreaker

import (
	. "github.com/idealo/aws-signing-proxy/pkg/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"os"
	"strconv"
	"strings"
	"time"
)

type CircuitBreaker struct {
	breaker *gobreaker.CircuitBreaker
}

func NewCircuitBreaker() *CircuitBreaker {

	timeout := getTimeout()
	failureThreshold := getFailureThreshold()

	st := gobreaker.Settings{
		Name:    "auth-circuit-breaker",
		Timeout: timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > failureThreshold
		},
	}
	return &CircuitBreaker{breaker: gobreaker.NewCircuitBreaker(st)}
}

func getFailureThreshold() uint32 {
	var failures uint32 = 5
	failuresStr, ok := os.LookupEnv("ASP_CIRCUIT_BREAKER_FAILURE_THRESHOLD")
	if ok {
		i, err := strconv.Atoi(failuresStr)
		if err != nil {
			Logger.Error("Failed parsing the circuit breaker failure count", zap.Error(err))
			return failures
		}
		return uint32(i)
	}
	return failures
}

func getTimeout() time.Duration {
	var timeout time.Duration
	var err error

	timeoutStr, ok := os.LookupEnv("ASP_CIRCUIT_BREAKER_TIMEOUT")
	if ok {
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			Logger.Error("Failed parsing the circuit breaker timeout", zap.Error(err))
			timeout = 0 // defaults to 60s
		}
	}
	return timeout
}

var (
	cbStateGauge   = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "auth_circuit_breaker_state", Help: "State of the authorization circuit breaker"}, []string{"state"})
	cbCounterGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "auth_circuit_breaker_count", Help: "Circuit breaker request count"}, []string{"type"})
)

func (cb *CircuitBreaker) Execute(req func() (interface{}, error)) (interface{}, error) {

	response, err := cb.breaker.Execute(req)

	switch cb.breaker.State() {
	case gobreaker.StateOpen:
		cbStateGauge.With(prometheus.Labels{"state": gobreaker.StateOpen.String()}).Set(1)
		cbStateGauge.With(prometheus.Labels{"state": gobreaker.StateClosed.String()}).Set(0)
		cbStateGauge.With(prometheus.Labels{"state": gobreaker.StateHalfOpen.String()}).Set(0)
	case gobreaker.StateHalfOpen:
		cbStateGauge.With(prometheus.Labels{"state": gobreaker.StateOpen.String()}).Set(0)
		cbStateGauge.With(prometheus.Labels{"state": gobreaker.StateHalfOpen.String()}).Set(1)
		cbStateGauge.With(prometheus.Labels{"state": gobreaker.StateClosed.String()}).Set(0)
	case gobreaker.StateClosed:
		cbStateGauge.With(prometheus.Labels{"state": gobreaker.StateOpen.String()}).Set(0)
		cbStateGauge.With(prometheus.Labels{"state": gobreaker.StateHalfOpen.String()}).Set(0)
		cbStateGauge.With(prometheus.Labels{"state": gobreaker.StateClosed.String()}).Set(1)
	}

	cbCounterGauge.WithLabelValues("requests").Set(float64(cb.breaker.Counts().Requests))
	cbCounterGauge.WithLabelValues("total_successes").Set(float64(cb.breaker.Counts().TotalSuccesses))
	cbCounterGauge.WithLabelValues("total_failures").Set(float64(cb.breaker.Counts().TotalFailures))
	cbCounterGauge.WithLabelValues("consecutive_successes").Set(float64(cb.breaker.Counts().ConsecutiveSuccesses))
	cbCounterGauge.WithLabelValues("consecutive_failures").Set(float64(cb.breaker.Counts().ConsecutiveFailures))

	if err != nil {
		if strings.Contains(err.Error(), "circuit breaker is open") {
			Logger.Warn(
				"Request to authorization server failed. Circuit breaker is open.",
				zap.String("name", cb.breaker.Name()),
				zap.String("state", cb.breaker.State().String()),
			)
			return response, err
		} else {
			Logger.Error("An error appeared", zap.Error(err))
			return response, err
		}
	}

	return response, err
}
