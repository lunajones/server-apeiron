package gameapi

import (
	"context"
	"testing"

	dbv1 "db-apeiron/gen/apeiron/v1"
	gamev1 "server-apeiron/gen/apeiron/game/v1"

	"google.golang.org/grpc"
)

type fakePlayerProgressionSource struct {
	player  *dbv1.Player
	found   bool
	updated []*dbv1.Player
}

func (f *fakePlayerProgressionSource) GetPlayer(ctx context.Context, in *dbv1.IdRequest, opts ...grpc.CallOption) (*dbv1.PlayerResponse, error) {
	return &dbv1.PlayerResponse{Found: f.found, Player: f.player}, nil
}

func (f *fakePlayerProgressionSource) UpdatePlayer(ctx context.Context, in *dbv1.Player, opts ...grpc.CallOption) (*dbv1.OperationResult, error) {
	f.updated = append(f.updated, in)
	return &dbv1.OperationResult{Success: true}, nil
}

// TestAttachLoadsPersistedPlayerProgression locks Progression Slice 1 (load): a player with non-default
// DB progression has it loaded onto the entity on attach, instead of resetting to hardcoded defaults.
func TestAttachLoadsPersistedPlayerProgression(t *testing.T) {
	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	runtime.SetPlayerProgressionSource(&fakePlayerProgressionSource{
		found: true,
		player: &dbv1.Player{
			Level: 7, Experience: 4200, AttributePoints: 9,
			Strength: 12, Dexterity: 5, Intelligence: 3, Endurance: 4, Coin: 150,
		},
	})

	if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{PlayerId: "p1"}); err != nil {
		t.Fatalf("attach: %v", err)
	}

	player := runtime.players["p1"]
	if player == nil || player.progression == nil {
		t.Fatal("player progression not set")
	}
	prog := player.progression
	if prog.level != 7 || prog.experience != 4200 || prog.attributePoints != 9 {
		t.Fatalf("level/xp/points = %d/%d/%d, want 7/4200/9", prog.level, prog.experience, prog.attributePoints)
	}
	if prog.strength != 12 || prog.dexterity != 5 || prog.intelligence != 3 || prog.endurance != 4 {
		t.Fatalf("attributes = %v/%v/%v/%v, want 12/5/3/4", prog.strength, prog.dexterity, prog.intelligence, prog.endurance)
	}
	if prog.coin != 150 {
		t.Fatalf("coin = %d, want 150", prog.coin)
	}
}

// TestAttachWithoutSourceKeepsDefaults ensures the nil-source path (tests / no db) keeps level-1 defaults.
func TestAttachWithoutSourceKeepsDefaults(t *testing.T) {
	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{PlayerId: "p1"}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	player := runtime.players["p1"]
	if player == nil || player.progression == nil {
		t.Fatal("player progression not set")
	}
	if player.progression.level != 1 {
		t.Fatalf("default level = %d, want 1", player.progression.level)
	}
}

// TestFlushPersistsDirtyProgression locks Progression Slice 1b (persist): an XP award marks the player
// dirty, and a flush writes it back via UpdatePlayer; a second flush with no change writes nothing.
func TestFlushPersistsDirtyProgression(t *testing.T) {
	fake := &fakePlayerProgressionSource{found: true, player: &dbv1.Player{Level: 1}}
	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	runtime.contracts.WolfPolicy.Progression = CreatureProgressionProfile{ExperienceValue: 100}
	runtime.SetPlayerProgressionSource(fake)

	player := runtime.ensurePlayerLocked("p1")
	wolf := runtime.ensureWolfLocked(player)
	runtime.creditDamageLocked(wolf, player, 50)
	wolf.health = 0
	runtime.awardKillXPLocked(wolf)

	runtime.flushDirtyProgression(context.Background())
	if len(fake.updated) != 1 {
		t.Fatalf("UpdatePlayer calls = %d, want 1", len(fake.updated))
	}
	if fake.updated[0].GetId() != "p1" || fake.updated[0].GetExperience() != 100 {
		t.Fatalf("persisted id %q xp %d, want p1/100", fake.updated[0].GetId(), fake.updated[0].GetExperience())
	}

	runtime.flushDirtyProgression(context.Background())
	if len(fake.updated) != 1 {
		t.Fatalf("second flush wrote again (%d); dirty not cleared", len(fake.updated))
	}
}
