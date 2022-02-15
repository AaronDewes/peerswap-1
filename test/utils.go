package test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/sputn1ck/peerswap/clightning"
	"github.com/sputn1ck/peerswap/peerswaprpc"
	"github.com/sputn1ck/peerswap/testframework"
	"github.com/stretchr/testify/require"
)

const defaultLines = 30

func IsIntegrationTest(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("set env RUN_INTEGRATION_TESTS=1 to run this test")
	}
}

type tailableProcess struct {
	p      *testframework.DaemonProcess
	lines  int
	filter string
}

func pprintFail(fps ...tailableProcess) {
	fmt.Printf("\n============================== FAILURE ==============================\n\n")
	for _, fp := range fps {
		if fp.p == nil {
			continue
		}
		fmt.Printf("+++++++++++++++++++++++++++++ %s (StdOut) +++++++++++++++++++++++++++++\n", fp.p.Prefix())
		fmt.Printf("%s\n", fp.p.StdOut.Tail(fp.lines, fp.filter))
		if fp.p.StdErr.String() != "" {
			fmt.Printf("+++++++++++++++++++++++++++++ %s (StdErr) +++++++++++++++++++++++++++++\n", fp.p.Prefix())
			fmt.Printf("%s\n", fp.p.StdErr.String())
		}
		fmt.Printf("+++++++++++++++++++++++++++++ %s (End) +++++++++++++++++++++++++++++\n", fp.p.Prefix())
		fmt.Printf("\n")
	}
}

type ChainNode interface {
	GenerateBlocks(b int) error
}

type pollableNode interface {
	GetId() string
	TriggerPoll() error
	AwaitPollFrom(node pollableNode) error
}

type clnPollableNode struct {
	*testframework.CLightningNode
}

func (n *clnPollableNode) GetId() string {
	return n.Id()
}

func (n *clnPollableNode) TriggerPoll() error {
	var result interface{}
	err := n.Rpc.Request(&clightning.ReloadPolicyFile{}, &result)
	if err != nil {
		return err
	}
	return nil
}

func (n *clnPollableNode) AwaitPollFrom(node pollableNode) error {
	return n.WaitForLog(fmt.Sprintf("From: %s got msgtype: a463", node.GetId()), testframework.TIMEOUT)
}

type peerswapPollableNode struct {
	*PeerSwapd
	peerId string
}

func (n *peerswapPollableNode) GetId() string {
	return n.peerId
}

func (n *peerswapPollableNode) TriggerPoll() error {
	_, err := n.PeerswapClient.ReloadPolicyFile(context.Background(), &peerswaprpc.ReloadPolicyFileRequest{})
	if err != nil {
		return err
	}
	return nil
}

func (n *peerswapPollableNode) AwaitPollFrom(node pollableNode) error {
	return n.WaitForLog(fmt.Sprintf("From: %s got msgtype: a463", node.GetId()), testframework.TIMEOUT)
}

func syncPoll(a, b pollableNode) error {
	go a.TriggerPoll()
	go b.TriggerPoll()

	err := a.AwaitPollFrom(b)
	if err != nil {
		return fmt.Errorf("AwaitPollFrom() (ab) %w", err)
	}

	err = b.AwaitPollFrom(a)
	if err != nil {
		return fmt.Errorf("AwaitPollFrom() (ba) %w", err)
	}

	return nil
}

func waitForBlockheightSync(t *testing.T, timeout time.Duration, nodes ...testframework.LightningNode) {
	for _, node := range nodes {
		err := testframework.WaitFor(func() bool {
			ok, err := node.IsBlockHeightSynced()
			require.NoError(t, err)
			return ok
		}, timeout)
		require.NoError(t, err)
	}
}

func shiftBalance(t *testing.T, from testframework.LightningNode, to testframework.LightningNode,
	scid string, amt uint64, batches int, timeout time.Duration) error {

	// We have to do this in batches as it would fail on big amounts.
	var inv string
	var err error
	for i := 0; i < batches; i++ {
		inv, err = to.AddInvoice(amt/uint64(batches), "shift balance", "")
		require.NoError(t, err)

		err = from.PayInvoice(inv)
		require.NoError(t, err)
	}
	return nil
}

func waitForTxInMempool(t *testing.T, chainRpc *testframework.RpcProxy, timeout time.Duration) (satFee uint64, err error) {
	err = testframework.WaitFor(func() bool {
		var mempool map[string]struct {
			Fees struct {
				Base float64 `json:"base"`
			} `json:"fees"`
		}
		jsonR, err := chainRpc.Call("getrawmempool", true)
		require.NoError(t, err)

		err = jsonR.GetObject(&mempool)
		require.NoError(t, err)

		if len(mempool) == 1 {
			for _, tx := range mempool {
				satFee = uint64(tx.Fees.Base * 100000000)
				return true
			}
		}
		return false
	}, timeout)
	return satFee, err
}
