package swap

import (
	"log"
	"testing"
	"time"

	"github.com/sputn1ck/glightning/glightning"
	"github.com/sputn1ck/peerswap/messages"
	"github.com/stretchr/testify/assert"
)

func Test_GoodCase(t *testing.T) {

	channelId := "chanId"
	amount := uint64(100)
	peer := "bob"
	initiator := "alice"

	aliceSwapService := getTestSetup("alice")
	bobSwapService := getTestSetup("bob")
	aliceSwapService.swapServices.messenger.(*ConnectedMessenger).other = bobSwapService.swapServices.messenger.(*ConnectedMessenger)
	bobSwapService.swapServices.messenger.(*ConnectedMessenger).other = aliceSwapService.swapServices.messenger.(*ConnectedMessenger)

	aliceSwapService.swapServices.messenger.(*ConnectedMessenger).msgReceivedChan = make(chan messages.MessageType)
	bobSwapService.swapServices.messenger.(*ConnectedMessenger).msgReceivedChan = make(chan messages.MessageType)

	aliceMsgChan := aliceSwapService.swapServices.messenger.(*ConnectedMessenger).msgReceivedChan
	bobMsgChan := bobSwapService.swapServices.messenger.(*ConnectedMessenger).msgReceivedChan

	err := aliceSwapService.Start()
	if err != nil {
		t.Fatal(err)
	}
	err = bobSwapService.Start()
	if err != nil {
		t.Fatal(err)
	}
	aliceSwap, err := aliceSwapService.SwapOut(peer, "l-btc", channelId, initiator, amount)
	if err != nil {
		t.Fatalf(" error swapping oput %v: ", err)
	}
	bobReceivedMsg := <-bobMsgChan
	assert.Equal(t, messages.MESSAGETYPE_SWAPOUTREQUEST, bobReceivedMsg)
	bobSwap := bobSwapService.activeSwaps[aliceSwap.Id]

	aliceReceivedMsg := <-aliceMsgChan
	assert.Equal(t, messages.MESSAGETYPE_SWAPOUTAGREEMENT, aliceReceivedMsg)

	assert.Equal(t, State_SwapOutSender_AwaitTxBroadcastedMessage, aliceSwap.Current)
	assert.Equal(t, State_SwapOutReceiver_AwaitFeeInvoicePayment, bobSwap.Current)

	bobSwapService.swapServices.lightning.(*dummyLightningClient).TriggerPayment(&glightning.Payment{
		Label: "fee_" + bobSwap.Id,
	})
	assert.Equal(t, State_SwapOutReceiver_AwaitClaimInvoicePayment, bobSwap.Current)

	aliceReceivedMsg = <-aliceMsgChan
	assert.Equal(t, messages.MESSAGETYPE_OPENINGTXBROADCASTED, aliceReceivedMsg)

	// trigger openingtx confirmed
	err = aliceSwapService.swapServices.liquidTxWatcher.(*dummyChain).txConfirmedFunc(aliceSwap.Id)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, State_ClaimedPreimage, aliceSwap.Current)

	// trigger bob payment received
	bobSwapService.swapServices.lightning.(*dummyLightningClient).TriggerPayment(&glightning.Payment{
		Label: "claim_" + bobSwap.Id,
	})
	assert.Equal(t, State_ClaimedPreimage, bobSwap.Current)
}
func Test_FeePaymentFailed(t *testing.T) {
	channelId := "chanId"
	amount := uint64(100)
	peer := "bob"
	initiator := "alice"

	aliceSwapService := getTestSetup("alice")
	bobSwapService := getTestSetup("bob")

	// set lightning to fail
	aliceSwapService.swapServices.lightning.(*dummyLightningClient).failpayment = true

	aliceSwapService.swapServices.messenger.(*ConnectedMessenger).other = bobSwapService.swapServices.messenger.(*ConnectedMessenger)
	bobSwapService.swapServices.messenger.(*ConnectedMessenger).other = aliceSwapService.swapServices.messenger.(*ConnectedMessenger)

	aliceSwapService.swapServices.messenger.(*ConnectedMessenger).msgReceivedChan = make(chan messages.MessageType)
	bobSwapService.swapServices.messenger.(*ConnectedMessenger).msgReceivedChan = make(chan messages.MessageType)

	aliceMsgChan := aliceSwapService.swapServices.messenger.(*ConnectedMessenger).msgReceivedChan
	bobMsgChan := bobSwapService.swapServices.messenger.(*ConnectedMessenger).msgReceivedChan

	err := aliceSwapService.Start()
	if err != nil {
		t.Fatal(err)
	}
	err = bobSwapService.Start()
	if err != nil {
		t.Fatal(err)
	}
	aliceSwap, err := aliceSwapService.SwapOut(peer, "btc", channelId, initiator, amount)
	if err != nil {
		t.Fatalf(" error swapping oput %v: ", err)
	}
	bobReceivedMsg := <-bobMsgChan
	assert.Equal(t, messages.MESSAGETYPE_SWAPOUTREQUEST, bobReceivedMsg)
	bobSwap, err := bobSwapService.GetActiveSwap(aliceSwap.Id)
	assert.NoError(t, err)

	aliceReceivedMsg := <-aliceMsgChan
	assert.Equal(t, messages.MESSAGETYPE_SWAPOUTAGREEMENT, aliceReceivedMsg)

	assert.Equal(t, State_SwapCanceled, aliceSwap.Current)

	bobReceivedMsg = <-bobMsgChan
	assert.Equal(t, messages.MESSAGETYPE_CANCELED, bobReceivedMsg)
	assert.Equal(t, State_SwapCanceled, bobSwap.Current)
}
func Test_ClaimPaymentFailedCoopClose(t *testing.T) {
	channelId := "chanId"
	amount := uint64(100)
	peer := "bob"
	initiator := "alice"

	aliceSwapService := getTestSetup("alice")
	bobSwapService := getTestSetup("bob")
	aliceSwapService.swapServices.messenger.(*ConnectedMessenger).other = bobSwapService.swapServices.messenger.(*ConnectedMessenger)
	bobSwapService.swapServices.messenger.(*ConnectedMessenger).other = aliceSwapService.swapServices.messenger.(*ConnectedMessenger)

	aliceSwapService.swapServices.messenger.(*ConnectedMessenger).msgReceivedChan = make(chan messages.MessageType)
	bobSwapService.swapServices.messenger.(*ConnectedMessenger).msgReceivedChan = make(chan messages.MessageType)

	aliceMsgChan := aliceSwapService.swapServices.messenger.(*ConnectedMessenger).msgReceivedChan
	bobMsgChan := bobSwapService.swapServices.messenger.(*ConnectedMessenger).msgReceivedChan

	err := aliceSwapService.Start()
	if err != nil {
		t.Fatal(err)
	}
	err = bobSwapService.Start()
	if err != nil {
		t.Fatal(err)
	}
	aliceSwap, err := aliceSwapService.SwapOut(peer, "btc", channelId, initiator, amount)
	if err != nil {
		t.Fatalf(" error swapping oput %v: ", err)
	}
	bobReceivedMsg := <-bobMsgChan
	assert.Equal(t, messages.MESSAGETYPE_SWAPOUTREQUEST, bobReceivedMsg)
	bobSwap := bobSwapService.activeSwaps[aliceSwap.Id]

	aliceReceivedMsg := <-aliceMsgChan
	assert.Equal(t, messages.MESSAGETYPE_SWAPOUTAGREEMENT, aliceReceivedMsg)

	assert.Equal(t, State_SwapOutSender_AwaitTxBroadcastedMessage, aliceSwap.Current)
	assert.Equal(t, State_SwapOutReceiver_AwaitFeeInvoicePayment, bobSwap.Current)

	bobSwapService.swapServices.lightning.(*dummyLightningClient).TriggerPayment(&glightning.Payment{
		Label: "fee_" + bobSwap.Id,
	})
	assert.Equal(t, State_SwapOutReceiver_AwaitClaimInvoicePayment, bobSwap.Current)

	aliceReceivedMsg = <-aliceMsgChan
	assert.Equal(t, messages.MESSAGETYPE_OPENINGTXBROADCASTED, aliceReceivedMsg)

	// trigger openingtx confirmed
	aliceSwapService.swapServices.lightning.(*dummyLightningClient).failpayment = true
	err = aliceSwapService.swapServices.liquidTxWatcher.(*dummyChain).txConfirmedFunc(aliceSwap.Id)
	if err != nil {
		t.Fatal(err)
	}
	// wants to await the csv claim before it goes to a
	// finish state, such that the channel is still
	// locked for furhter peerswap requests.
	assert.Equal(t, State_ClaimedCoop, aliceSwap.Current)

	// trigger bob payment received

	bobReceivedMsg = <-bobMsgChan
	assert.Equal(t, messages.MESSAGETYPE_COOPCLOSE, bobReceivedMsg)
	assert.Equal(t, State_ClaimedCoop, bobSwap.Current)
}

func Test_OnlyOneActiveSwapPerChannel(t *testing.T) {
	service := getTestSetup("alice")
	service.AddActiveSwap("swapid", &SwapStateMachine{
		Id: "swapid",
		Data: &SwapData{
			Id:                     "swapid",
			Type:                   0,
			FSMState:               "",
			Role:                   0,
			CreatedAt:              0,
			InitiatorNodeId:        "",
			PeerNodeId:             "",
			Amount:                 0,
			ChannelId:              "channelID",
			PrivkeyBytes:           []byte{},
			ClaimInvoice:           "",
			ClaimPreimage:          "",
			ClaimPaymentHash:       "",
			MakerPubkeyHash:        "",
			TakerPubkeyHash:        "",
			FeeInvoice:             "",
			FeePreimage:            "",
			OpeningTxId:            "",
			OpeningTxUnpreparedHex: "",
			OpeningTxVout:          0,
			OpeningTxFee:           0,
			OpeningTxHex:           "",
			ClaimTxId:              "",
			CancelMessage:          "",
			LastErr:                nil,
			LastErrString:          "",
		},
		Type:     0,
		Role:     0,
		Previous: "",
		Current:  "",
		States: map[StateType]State{
			"": {
				Action: nil,
				Events: map[EventType]StateType{
					"": "",
				},
			},
		},
		swapServices: &SwapServices{
			swapStore:        nil,
			lightning:        nil,
			messenger:        nil,
			policy:           nil,
			bitcoinTxWatcher: nil,
			liquidTxWatcher:  nil,
		},
		retries:  0,
		failures: 0,
	})

	_, err := service.SwapOut("peer", "l-btc", "channelID", "alice", uint64(200))
	if assert.Error(t, err, "expected error") {
		assert.Equal(t, "already has an active swap on channel", err.Error())
	}

	_, err = service.SwapIn("peer", "l-btc", "channelID", "alice", uint64(200))
	if assert.Error(t, err, "expected error") {
		assert.Equal(t, "already has an active swap on channel", err.Error())
	}
}

func getTestSetup(name string) *SwapService {
	store := &dummyStore{dataMap: map[string]*SwapStateMachine{}}
	reqSwapsStore := &requestedSwapsStoreMock{data: map[string][]RequestedSwap{}}
	messenger := &ConnectedMessenger{
		thisPeerId: name,
	}
	lc := &dummyLightningClient{preimage: ""}
	policy := &dummyPolicy{}
	chain := &dummyChain{}
	swapServices := NewSwapServices(store, reqSwapsStore, lc, messenger, policy, true, chain, chain, chain, true, chain, chain, chain)
	swapService := NewSwapService(swapServices)
	return swapService
}

type ConnectedMessenger struct {
	thisPeerId      string
	OnMessage       func(peerId string, msgType string, msgBytes []byte) error
	other           *ConnectedMessenger
	msgReceivedChan chan messages.MessageType
}

func (c *ConnectedMessenger) SendMessage(peerId string, msg []byte, msgType int) error {
	go func() {
		time.Sleep(time.Millisecond * 10)
		msgString := messages.MessageTypeToHexString(messages.MessageType(msgType))
		err := c.other.OnMessage(c.thisPeerId, msgString, msg)
		if err != nil {
			log.Printf("error on message send %v", err)
		}
		if c.other.msgReceivedChan != nil {
			c.other.msgReceivedChan <- messages.MessageType(msgType)
		}
	}()

	return nil
}

func (c *ConnectedMessenger) AddMessageHandler(f func(peerId string, msgType string, msgBytes []byte) error) {
	c.OnMessage = f
}
