package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/dapr/go-sdk/actor"
	dapr "github.com/dapr/go-sdk/client"
	"github.com/dapr/go-sdk/service/common"
	daprd "github.com/dapr/go-sdk/service/http"
	"github.com/mitchellh/mapstructure"
)

// **** MODELS ****//
type CampaignStats struct {
	CampaignId        string
	GoalUSD           int64
	GoalEUR           int64
	ToTargetUSD       int64
	ToTargetEUR       int64
	Donors            int32
	Pledges           int32
	LargetPledgeUSD   int64
	LargetPledgeEUR   int64
	MostGenerousDonor string
	AvgPledgeUSD      int64
	AvgPledgeEUR      int64
	PledgedAmountUSD  int64
	PledgedAmountEUR  int64
}

type PledgeEvent struct {
	CampaignId string
	Time       time.Time
	Donor      string
	Amount     float64
	Currency   string
}

// **** ACTOR API ****//
type CampaignActorStub struct {
	Id     string
	Pledge func(context.Context, PledgeEvent) error
}

func NewCampaignActor(id string) *CampaignActorStub {
	return &CampaignActorStub{Id: id}
}

func (a *CampaignActorStub) Type() string {
	return "CampaignActorType"
}

func (a *CampaignActorStub) ID() string {
	return a.Id
}

// **** ACTOR IMPL ****//
func CampaignActorFactory() actor.ServerContext {
	return &CampaignActor{}
}

type CampaignActor struct {
	// Platform dependencies
	actor.ServerImplBaseCtx

	// Main State
	mainState mainState

	// Pledges State
	pledgesState pledgesState
}

func (a *CampaignActor) Type() string {
	return "CampaignActorType"
}

func (a *CampaignActor) Pledge(ctx context.Context, evt PledgeEvent) error {
	fmt.Println("CampaignActor", "pledge")
	a.getState(ctx, mainStateKey)
	a.mainState.PledgesCounter++
	a.mainState.Amounts[evt.Currency] += evt.Amount
	a.saveState(ctx, mainStateKey)

	a.getState(ctx, pledgesStateKey)
	// The actor should limit the pledges to the last 10 for example
	a.pledgesState.Pledges = append(a.pledgesState.Pledges, evt)
	a.saveState(ctx, pledgesStateKey)

	return nil
}

// **** ACTOR STATE ****//
const (
	mainStateKey    string = "main"
	pledgesStateKey string = "pledges"
)

type mainState struct {
	Name           string             `json:"name"`
	PledgesCounter int                `json:"pledges"`
	Amounts        map[string]float64 `json:"amounts"`
}

type pledgesState struct {
	Name    string        `json:"name"`
	Pledges []PledgeEvent `json:"pledges"` // Realistically this should contain the last 10 pledges, for example
}

func (a *CampaignActor) saveState(ctx context.Context, stateKey string) {
	if stateKey == mainStateKey {
		a.GetStateManager().Set(ctx, stateKey, a.mainState)
	}

	if stateKey == pledgesStateKey {
		a.GetStateManager().Set(ctx, stateKey, a.pledgesState)
	}

	a.GetStateManager().Save(ctx)
}

func (a *CampaignActor) getState(ctx context.Context, stateKey string) {
	exist, err := a.GetStateManager().Contains(ctx, stateKey)
	if err != nil {
		fmt.Println("getState error", err)
		return
	}

	// If the state does not exist, initialize a new state struct and return it
	if !exist && stateKey == mainStateKey {
		a.mainState = mainState{
			Name:           "Main State",
			PledgesCounter: 0,
			Amounts:        map[string]float64{},
		}
		return
	}

	if !exist && stateKey == mainStateKey {
		a.pledgesState = pledgesState{
			Name:    "Pledges State",
			Pledges: []PledgeEvent{},
		}
		return
	}

	// If the state exists, retrieve from store into the actor struct
	if stateKey == mainStateKey {
		a.GetStateManager().Get(ctx, stateKey, &a.mainState)
		return
	}

	if stateKey == pledgesStateKey {
		a.GetStateManager().Get(ctx, stateKey, &a.pledgesState)
	}
}

// **** UTILS ****//
func init() {
	rand.Seed(time.Now().UnixNano())
}

var letters = []rune("0123456789ABCDEFGHIJKLMNOPQRSTWXYZ")
var numbers = []rune("0123456789")

func RandString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}

func RandNumber(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = numbers[rand.Intn(len(letters))]
	}

	return string(b)
}

func RandInBetween(min, max int) int {
	return rand.Intn(max-min) + min
}

// **** MAIN ****//
const (
	CAMPAIGN_PUB_SUB = "campaign-pubsub" // name must match config/redis-pubsub.yaml
	PLEDGE_TOPIC     = "pledge-topic"
)

var topicSubscription = &common.Subscription{
	PubsubName: CAMPAIGN_PUB_SUB,
	Topic:      PLEDGE_TOPIC,
	Route:      fmt.Sprintf("/%s", PLEDGE_TOPIC),
}

// Global DAPR client
var daprclient dapr.Client

func main() {
	rootCtx := context.Background()
	canxCtx, _ := signal.NotifyContext(rootCtx, os.Interrupt)

	// Create a DAPR service using a hard-coded port (must match start-core.sh)
	s := daprd.NewService(":8080")
	fmt.Println("DAPR Service created!")

	// Create a DAPR client
	// Must be a global client since it is singleton
	// Hence it would be injected in actor packages as needed
	c, err := dapr.NewClient()
	if err != nil {
		fmt.Println("Failed to start dapr client", err)
		return
	}
	daprclient = c
	defer daprclient.Close()

	// Register actors
	s.RegisterActorImplFactoryContext(CampaignActorFactory)
	fmt.Println("Campaign actor registered!")

	// Register pub/sub pledge handler
	if err := s.AddTopicEventHandler(topicSubscription, pledgeHandler); err != nil {
		panic(err)
	}
	fmt.Println("Campaign topic handler registered!")

	// Launch a concurrent routine to pump events to topic
	go func() {
		for {
			select {
			case <-canxCtx.Done():
				fmt.Println("events pumper context cancelled")
				return
			case <-time.After(time.Duration(5) * time.Second):
				fmt.Println("events pumper woke up")
				e := PledgeEvent{
					CampaignId: fmt.Sprintf("%d", RandInBetween(1000, 1003)),
					Donor:      donor(),
					Time:       time.Now(),
					Amount:     float64(RandInBetween(10, 1000)),
					Currency:   currency(),
				}
				err = daprclient.PublishEvent(canxCtx, CAMPAIGN_PUB_SUB, PLEDGE_TOPIC, e)
				if err != nil {
					fmt.Println("events pumper published errored out", err)
				}
			}
		}
	}()

	// Start DAPR service
	if err := s.Start(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

func pledgeHandler(ctx context.Context, e *common.TopicEvent) (retry bool, err error) {
	fmt.Println("pledgeHandler....")
	// Decode event
	evt := PledgeEvent{}
	mapstructure.Decode(e.Data, &evt)

	fmt.Printf("Received a pledge for campaign %s - amount %f - currency %s\n",
		evt.CampaignId,
		evt.Amount,
		evt.Currency)

	// Resolve actor by campaign id
	campaignActorProxy := NewCampaignActor(evt.CampaignId)
	daprclient.ImplActorClientStub(campaignActorProxy)

	// Call actor methods
	err = campaignActorProxy.Pledge(ctx, evt)
	if err != nil {
		return false, err
	}

	return false, nil
}

func currency() string {
	i := RandInBetween(0, 2)
	if i == 1 {
		return "EUR"
	}

	return "USD"
}

func donor() string {
	return RandString(4) + " " + RandString(10)
}
