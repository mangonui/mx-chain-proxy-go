package process

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/multiversx/mx-chain-core-go/core"
	"github.com/multiversx/mx-chain-proxy-go/process/mock"
	"github.com/stretchr/testify/require"
)

func createMockArgNumShardsProcessor() ArgNumShardsProcessor {
	return ArgNumShardsProcessor{
		HttpClient:                    &mock.HttpClientMock{},
		Observers:                     []string{"obs1, obs2"},
		TimeBetweenNodesRequestsInSec: 2,
		NumShardsTimeoutInSec:         10,
		RequestTimeoutInSec:           5,
	}
}

func makeNumShardsProcessorWithTestDurations(t *testing.T, args ArgNumShardsProcessor, between, timeout, request time.Duration) *numShardsProcessor {
	t.Helper()

	proc, err := NewNumShardsProcessor(args)
	require.NoError(t, err)

	proc.timeBetweenNodesRequests = between
	proc.numShardsTimeout = timeout
	proc.requestTimeout = request

	return proc
}

func TestNewNumShardsProcessor(t *testing.T) {
	t.Parallel()

	t.Run("nil HttpClient should error", func(t *testing.T) {
		t.Parallel()

		args := createMockArgNumShardsProcessor()
		args.HttpClient = nil

		proc, err := NewNumShardsProcessor(args)
		require.Equal(t, ErrNilHttpClient, err)
		require.Nil(t, proc)
	})
	t.Run("empty observers list should error", func(t *testing.T) {
		t.Parallel()

		args := createMockArgNumShardsProcessor()
		args.Observers = []string{}

		proc, err := NewNumShardsProcessor(args)
		require.True(t, errors.Is(err, core.ErrInvalidValue))
		require.True(t, strings.Contains(err.Error(), "Observers"))
		require.Nil(t, proc)
	})
	t.Run("invalid TimeBetweenNodesRequestsInSec should error", func(t *testing.T) {
		t.Parallel()

		args := createMockArgNumShardsProcessor()
		args.TimeBetweenNodesRequestsInSec = 0

		proc, err := NewNumShardsProcessor(args)
		require.True(t, errors.Is(err, core.ErrInvalidValue))
		require.True(t, strings.Contains(err.Error(), "TimeBetweenNodesRequestsInSec"))
		require.Nil(t, proc)
	})
	t.Run("invalid NumShardsTimeoutInSec should error", func(t *testing.T) {
		t.Parallel()

		args := createMockArgNumShardsProcessor()
		args.NumShardsTimeoutInSec = 0

		proc, err := NewNumShardsProcessor(args)
		require.True(t, errors.Is(err, core.ErrInvalidValue))
		require.True(t, strings.Contains(err.Error(), "NumShardsTimeoutInSec"))
		require.Nil(t, proc)
	})
	t.Run("invalid RequestTimeoutInSec should error", func(t *testing.T) {
		t.Parallel()

		args := createMockArgNumShardsProcessor()
		args.RequestTimeoutInSec = 0

		proc, err := NewNumShardsProcessor(args)
		require.True(t, errors.Is(err, core.ErrInvalidValue))
		require.True(t, strings.Contains(err.Error(), "RequestTimeoutInSec"))
		require.Nil(t, proc)
	})
	t.Run("should work", func(t *testing.T) {
		t.Parallel()

		proc, err := NewNumShardsProcessor(createMockArgNumShardsProcessor())
		require.NoError(t, err)
		require.NotNil(t, proc)
	})
}

func TestNumShardsProcessor_GetNetworkNumShards(t *testing.T) {
	t.Parallel()

	t.Run("context done should exit with timeout", func(t *testing.T) {
		t.Parallel()

		args := createMockArgNumShardsProcessor()
		proc := makeNumShardsProcessorWithTestDurations(t, args, 10*time.Millisecond, 50*time.Millisecond, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		numShards, err := proc.GetNetworkNumShards(ctx)
		require.Equal(t, errTimeIsOut, err)
		require.Zero(t, numShards)
	})
	t.Run("timeout should exit with timeout", func(t *testing.T) {
		t.Parallel()

		args := createMockArgNumShardsProcessor()
		proc := makeNumShardsProcessorWithTestDurations(t, args, 10*time.Millisecond, 15*time.Millisecond, 10*time.Millisecond)

		numShards, err := proc.GetNetworkNumShards(context.Background())
		require.True(t, errors.Is(err, errTimeIsOut))
		require.Zero(t, numShards)
	})
	t.Run("should work on 4th observer", func(t *testing.T) {
		t.Parallel()

		providedBody := &networkConfigResponse{
			Data: networkConfigResponseData{
				Config: struct {
					NumShards uint32 `json:"erd_num_shards_without_meta"`
				}(struct{ NumShards uint32 }{NumShards: 2}),
			},
		}
		providedBodyBuff, _ := json.Marshal(providedBody)

		args := createMockArgNumShardsProcessor()
		cnt := 0
		args.HttpClient = &mock.HttpClientMock{
			DoCalled: func(req *http.Request) (*http.Response, error) {
				cnt++
				switch cnt {
				case 1: // error on Do
					return nil, errors.New("observer offline")
				case 2: // status code not 200
					return &http.Response{
						StatusCode: http.StatusBadRequest,
					}, nil
				case 3: // status code ok, but invalid response
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("not the expected response")),
					}, nil
				default: // response ok
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(providedBodyBuff)),
					}, nil
				}
			},
		}

		proc := makeNumShardsProcessorWithTestDurations(t, args, 5*time.Millisecond, 250*time.Millisecond, 50*time.Millisecond)
		numShards, err := proc.GetNetworkNumShards(context.Background())
		require.NoError(t, err)
		require.Equal(t, uint32(2), numShards)
	})
	t.Run("oversized_response_body_should_be_rejected", func(t *testing.T) {
		t.Parallel()

		args := createMockArgNumShardsProcessor()
		args.HttpClient = &mock.HttpClientMock{
			DoCalled: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), 5<<20))),
				}, nil
			},
		}

		proc := makeNumShardsProcessorWithTestDurations(t, args, 2*time.Millisecond, 10*time.Millisecond, 10*time.Millisecond)

		numShards, err := proc.GetNetworkNumShards(context.Background())
		require.ErrorIs(t, err, errTimeIsOut)
		require.Zero(t, numShards)
	})
}

func TestNumShardsProcessor_TryGetnumShardsFromObserverBodyTooLarge(t *testing.T) {
	t.Parallel()

	args := createMockArgNumShardsProcessor()
	args.HttpClient = &mock.HttpClientMock{
		DoCalled: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), 5<<20))),
			}, nil
		},
	}

	proc, err := NewNumShardsProcessor(args)
	require.NoError(t, err)

	numShards, statusCode := proc.tryGetnumShardsFromObserver("http://observer")
	require.Zero(t, numShards)
	require.Equal(t, http.StatusInternalServerError, statusCode)
}
