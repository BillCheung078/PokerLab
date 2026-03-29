package sim

import (
	"context"
	"fmt"
	"time"
)

const (
	// DefaultIntraHandDelay controls event pacing within one hand.
	DefaultIntraHandDelay = 550 * time.Millisecond
	// DefaultHandPause controls the pause between completed hands.
	DefaultHandPause = 1700 * time.Millisecond
)

// EngineConfig controls pacing for the simulated hand loop.
type EngineConfig struct {
	IntraHandDelay time.Duration
	HandPause      time.Duration
}

// Engine drives one independent simulation loop per table runtime.
type Engine struct {
	config EngineConfig
	now    func() time.Time
	sleep  func(context.Context, time.Duration) error
}

// NewEngine constructs the default local-development simulator.
func NewEngine() *Engine {
	return NewEngineWithConfig(EngineConfig{
		IntraHandDelay: DefaultIntraHandDelay,
		HandPause:      DefaultHandPause,
	})
}

// NewEngineWithConfig constructs an engine with explicit pacing controls.
func NewEngineWithConfig(config EngineConfig) *Engine {
	if config.IntraHandDelay <= 0 {
		config.IntraHandDelay = DefaultIntraHandDelay
	}
	if config.HandPause <= 0 {
		config.HandPause = DefaultHandPause
	}

	return &Engine{
		config: config,
		now:    func() time.Time { return time.Now().UTC() },
		sleep:  sleepWithContext,
	}
}

// Start launches the simulation loop for one runtime once.
func (e *Engine) Start(runtime *TableRuntime) {
	if runtime == nil {
		return
	}

	runtime.startOnce.Do(func() {
		go e.run(runtime)
	})
}

func (e *Engine) run(runtime *TableRuntime) {
	handNumber := 1

	for {
		if runtime.Context().Err() != nil {
			return
		}
		if err := e.playHand(runtime, handNumber); err != nil {
			return
		}
		handNumber++
		if err := e.sleep(runtime.Context(), e.config.HandPause); err != nil {
			return
		}
	}
}

func (e *Engine) playHand(runtime *TableRuntime, handNumber int) error {
	players := defaultPlayers()
	blindLevel := blindLevelForHand(handNumber)

	runtime.UpdateState(func(state *TableState) {
		state.GameNumber = handNumber
		state.BlindLevel = blindLevel
		state.Players = append([]PlayerSeat(nil), players...)
		state.CommunityCards = nil
		state.Status = "starting_hand"
	})

	if err := e.emit(runtime, "game_started", "starting_hand", map[string]any{
		"game_number": handNumber,
		"blind_level": blindLevel,
	}); err != nil {
		return err
	}

	if err := e.emit(runtime, "players_joined", "players_ready", map[string]any{
		"player_count": len(players),
		"players":      playerNames(players),
	}); err != nil {
		return err
	}

	pockets := pocketCardsForHand(handNumber)
	for index, seat := range players {
		if err := e.emit(runtime, "card_dealt", "preflop", map[string]any{
			"seat":  seat.Seat,
			"cards": pockets[index],
		}); err != nil {
			return err
		}
	}

	if err := e.emit(runtime, "bet_action", "preflop_betting", map[string]any{
		"seat":   1,
		"action": "small_blind",
		"amount": 1,
	}); err != nil {
		return err
	}

	if err := e.emit(runtime, "bet_action", "preflop_betting", map[string]any{
		"seat":   2,
		"action": "big_blind",
		"amount": 2,
	}); err != nil {
		return err
	}

	flop, turn, river := boardForHand(handNumber)
	runtime.UpdateState(func(state *TableState) {
		state.CommunityCards = append([]string(nil), flop...)
		state.Status = "flop"
	})
	if err := e.emit(runtime, "community_cards", "flop", map[string]any{
		"street": "flop",
		"cards":  flop,
	}); err != nil {
		return err
	}

	if err := e.emit(runtime, "bet_action", "flop_betting", map[string]any{
		"seat":   3,
		"action": "check",
	}); err != nil {
		return err
	}

	boardTurn := append(append([]string(nil), flop...), turn)
	runtime.UpdateState(func(state *TableState) {
		state.CommunityCards = boardTurn
		state.Status = "turn"
	})
	if err := e.emit(runtime, "community_cards", "turn", map[string]any{
		"street": "turn",
		"cards":  boardTurn,
	}); err != nil {
		return err
	}

	if err := e.emit(runtime, "bet_action", "turn_betting", map[string]any{
		"seat":   4,
		"action": "bet",
		"amount": 6,
	}); err != nil {
		return err
	}

	boardRiver := append(append([]string(nil), boardTurn...), river)
	runtime.UpdateState(func(state *TableState) {
		state.CommunityCards = boardRiver
		state.Status = "river"
	})
	if err := e.emit(runtime, "community_cards", "river", map[string]any{
		"street": "river",
		"cards":  boardRiver,
	}); err != nil {
		return err
	}

	runtime.UpdateState(func(state *TableState) {
		state.Status = "hand_complete"
	})
	if err := e.emit(runtime, "hand_result", "hand_complete", map[string]any{
		"winner_seat": 1 + ((handNumber - 1) % len(players)),
		"summary":     fmt.Sprintf("Hand %d complete", handNumber),
	}); err != nil {
		return err
	}

	return nil
}

func (e *Engine) emit(runtime *TableRuntime, eventType, status string, payload map[string]any) error {
	if runtime.Context().Err() != nil {
		return runtime.Context().Err()
	}

	runtime.UpdateState(func(state *TableState) {
		state.Status = status
	})

	runtime.AppendEvent(PokerEvent{
		Type:    eventType,
		Payload: payload,
		At:      e.now(),
	})

	return e.sleep(runtime.Context(), e.config.IntraHandDelay)
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func defaultPlayers() []PlayerSeat {
	return []PlayerSeat{
		{Seat: 1, Name: "Avery", Stack: 100, Status: "active"},
		{Seat: 2, Name: "Blake", Stack: 100, Status: "active"},
		{Seat: 3, Name: "Casey", Stack: 100, Status: "active"},
		{Seat: 4, Name: "Devon", Stack: 100, Status: "active"},
	}
}

func playerNames(players []PlayerSeat) []string {
	names := make([]string, 0, len(players))
	for _, player := range players {
		names = append(names, player.Name)
	}

	return names
}

func blindLevelForHand(handNumber int) string {
	levels := []string{"1/2", "2/4", "3/6", "5/10"}
	return levels[(handNumber-1)%len(levels)]
}

func pocketCardsForHand(handNumber int) [][]string {
	cards := [][]string{
		{"Ah", "Kd"},
		{"Qs", "Qd"},
		{"Jc", "10c"},
		{"9h", "9c"},
	}

	shift := (handNumber - 1) % len(cards)
	result := make([][]string, 0, len(cards))
	for i := 0; i < len(cards); i++ {
		pair := cards[(i+shift)%len(cards)]
		result = append(result, append([]string(nil), pair...))
	}

	return result
}

func boardForHand(handNumber int) ([]string, string, string) {
	boards := []struct {
		flop  []string
		turn  string
		river string
	}{
		{flop: []string{"As", "7d", "2c"}, turn: "Kh", river: "3s"},
		{flop: []string{"Qc", "Jh", "4s"}, turn: "10d", river: "2h"},
		{flop: []string{"9d", "8c", "5h"}, turn: "Ad", river: "Kc"},
		{flop: []string{"7s", "6s", "4d"}, turn: "Qh", river: "Jd"},
	}

	board := boards[(handNumber-1)%len(boards)]
	return append([]string(nil), board.flop...), board.turn, board.river
}
