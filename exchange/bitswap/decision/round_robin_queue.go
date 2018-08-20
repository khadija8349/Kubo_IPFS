// Round-Robin Queue
// =================

package decision

import (
	"math"

	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
)

// Types and Constructors
// ----------------------

type RRQConfig struct {
	RoundBurst int
	Strategy   Strategy
}

type RRPeer struct {
	id         peer.ID
	allocation int
}

// Round Robin Queue
type RRQueue struct {
	roundBurst  int
	strategy    Strategy
	weights     map[peer.ID]float64
	allocations []*RRPeer
}

func newRRQueue(cfg *RRQConfig) *RRQueue {
	return &RRQueue{
		roundBurst:  cfg.RoundBurst,
		strategy:    cfg.Strategy,
		weights:     make(map[peer.ID]float64),
		allocations: []*RRPeer{},
	}
}

// Peer Management
// ---------------

func (rrq *RRQueue) InitRound() {
	totalWeight := float64(0)
	for _, weight := range rrq.weights {
		totalWeight += weight
	}

	for id, weight := range rrq.weights {
		allocation := int((weight / totalWeight) * float64(rrq.roundBurst))
		if allocation <= 0 {
			continue
		}
		rrp := &RRPeer{
			id:         id,
			allocation: allocation,
		}
		rrq.allocations = append(rrq.allocations, rrp)
	}
}

// update peer's weight using their current receipt
func (rrq *RRQueue) UpdateWeight(id peer.ID, r *Receipt) {
	rrq.weights[id] = rrq.strategy(r)
}

func (rrq *RRQueue) Pop() {
	if len(rrq.allocations) != 0 {
		rrq.allocations = rrq.allocations[1:]
	}
}

func (rrq *RRQueue) Head() *RRPeer {
	if len(rrq.allocations) == 0 {
		return nil
	}
	return rrq.allocations[0]
}

func (rrq *RRQueue) Shift() {
	rrq.allocations = append(rrq.allocations[1:], rrq.allocations[0])
}

func (rrq *RRQueue) ResetAllocations() {
	rrq.allocations = []*RRPeer{}
}

// Utility Functions
// -----------------

func (rrq *RRQueue) NumPeers() int {
	return len(rrq.allocations)
}

// Strategy
// --------

// takes in a peer's ledger, returns RR weight for that peer
type Strategy func(r *Receipt) float64

// simple weighting function based on peer's ledger Value
func Identity(r *Receipt) float64 {
	if r.Value <= 0 {
		return 0
	}
	return r.Value
}

func Sigmoid(r *Receipt) float64 {
	return 1 / (1 + math.Exp(2-r.Value))
}

func Tanh(r *Receipt) float64 {
	return math.Tanh(r.Value)
}
