package process

import (
	"net/http"
	"testing"
	"time"

	"github.com/multiversx/mx-chain-proxy-go/process/mock"
	"github.com/stretchr/testify/require"
)

func TestNewBaseProcessor_DoesNotMutateDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	originalTimeout := http.DefaultClient.Timeout
	http.DefaultClient.Timeout = 0
	t.Cleanup(func() {
		http.DefaultClient.Timeout = originalTimeout
	})

	bp, err := NewBaseProcessor(
		5,
		&mock.ShardCoordinatorMock{},
		&mock.ObserversProviderStub{},
		&mock.ObserversProviderStub{},
		&mock.PubKeyConverterMock{},
		false,
	)
	require.NoError(t, err)
	require.NotNil(t, bp)
	require.Equal(t, 5*time.Second, bp.httpClient.Timeout)
	require.Equal(t, time.Duration(0), http.DefaultClient.Timeout)
	require.NotSame(t, http.DefaultClient, bp.httpClient)
}
