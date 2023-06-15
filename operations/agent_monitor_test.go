package operations

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/agent"
	"github.com/mongodb/jasper"
	"github.com/mongodb/jasper/options"
	"github.com/mongodb/jasper/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const monitorTestTimeout = 10 * time.Second

func TestAgentMonitorWithJasper(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jasperPort := evergreen.DefaultJasperPort
	port := defaultMonitorPort
	manager, err := jasper.NewSynchronizedManager(false)
	require.NoError(t, err)
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%d", jasperPort))
	require.NoError(t, err)
	closeServer, err := remote.StartRPCService(ctx, manager, addr, nil)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, closeServer())
	}()

	for testName, testCase := range map[string]func(context.Context, *testing.T, *monitor){
		"FetchClientDownloadsFromURL": func(ctx context.Context, t *testing.T, m *monitor) {
			require.NoError(t, m.fetchClient(ctx, []string{"https://example.com"}, agentMonitorDefaultRetryOptions()))
			fileInfo, err := os.Stat(m.clientPath)
			require.NoError(t, err)
			assert.NotZero(t, fileInfo.Size())
		},
		"WaitUntilCompleteWaitsForProcessTermination": func(ctx context.Context, t *testing.T, m *monitor) {
			opts := &options.Create{Args: []string{"sleep", "1"}}
			proc, err := m.jasperClient.CreateProcess(ctx, opts)
			require.NoError(t, err)
			exitCode, err := waitUntilComplete(ctx, proc, time.Second)
			require.NoError(t, err)
			assert.True(t, proc.Complete(ctx))
			assert.Zero(t, exitCode)
		},
	} {
		t.Run(testName, func(t *testing.T) {
			tctx, tcancel := context.WithTimeout(ctx, monitorTestTimeout)
			defer tcancel()

			tmpDir := t.TempDir()

			m := &monitor{
				clientPath: filepath.Join(tmpDir, "evergreen"),
				distroID:   "distro",
				logOutput:  agent.LogOutputFile,
				logPrefix:  filepath.Join(tmpDir, "agent-monitor"),
				jasperPort: jasperPort,
				port:       port,
			}

			// Monitor should be able to connect without needing credentials when
			// testing.
			require.NoError(t, m.setupJasperConnection(tctx, agentMonitorDefaultRetryOptions()))
			defer func() {
				assert.NoError(t, m.jasperClient.CloseConnection())
			}()

			testCase(tctx, t, m)
		})
	}
}
