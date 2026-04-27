package process

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/multiversx/mx-chain-proxy-go/process/mock"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestBaseProcessor_CallGetRestEndPointBodyTooLarge(t *testing.T) {
	t.Parallel()

	bp, err := NewBaseProcessor(
		5,
		&mock.ShardCoordinatorMock{},
		&mock.ObserversProviderStub{},
		&mock.ObserversProviderStub{},
		&mock.PubKeyConverterMock{},
		false,
	)
	require.NoError(t, err)

	bp.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), 5<<20))),
			}, nil
		}),
	}

	var response map[string]interface{}
	statusCode, err := bp.CallGetRestEndPoint("http://observer", "/path", &response)

	require.Equal(t, http.StatusInternalServerError, statusCode)
	require.ErrorIs(t, err, ErrResponseBodyTooLarge)
}

func TestBaseProcessor_CallPostRestEndPointBodyTooLarge(t *testing.T) {
	t.Parallel()

	bp, err := NewBaseProcessor(
		5,
		&mock.ShardCoordinatorMock{},
		&mock.ObserversProviderStub{},
		&mock.ObserversProviderStub{},
		&mock.PubKeyConverterMock{},
		false,
	)
	require.NoError(t, err)

	bp.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), 5<<20))),
			}, nil
		}),
	}

	var response map[string]interface{}
	statusCode, err := bp.CallPostRestEndPoint("http://observer", "/path", map[string]string{"a": "b"}, &response)

	require.Equal(t, http.StatusInternalServerError, statusCode)
	require.ErrorIs(t, err, ErrResponseBodyTooLarge)
}

func TestReadResponseBodyLimitedRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	_, err := readResponseBodyLimited(bytes.NewReader(bytes.Repeat([]byte("a"), 5<<20)))
	require.ErrorIs(t, err, ErrResponseBodyTooLarge)
}

func TestReadResponseBodyLimitedAcceptsBoundarySizedBody(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("a", maxUpstreamResponseBodyBytes)
	readBody, err := readResponseBodyLimited(strings.NewReader(body))
	require.NoError(t, err)
	require.Len(t, readBody, maxUpstreamResponseBodyBytes)
}

func TestReadResponseBodyLimitedPropagatesReaderError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("read failed")
	_, err := readResponseBodyLimited(errReader{err: expectedErr})
	require.ErrorIs(t, err, expectedErr)
}

type errReader struct {
	err error
}

func (e errReader) Read(_ []byte) (int, error) {
	return 0, e.err
}
