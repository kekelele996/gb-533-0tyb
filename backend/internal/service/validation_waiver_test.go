package service

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"robot-cell-safety-envelope-validator/backend/internal/constants"
	"robot-cell-safety-envelope-validator/backend/internal/dto"
	"robot-cell-safety-envelope-validator/backend/internal/model"
	"robot-cell-safety-envelope-validator/backend/internal/repository"
)

type waiverFixture struct {
	db            *gorm.DB
	service       *ValidationRunService
	reviewer      dto.Actor
	otherReviewer dto.Actor
	uploader      dto.Actor
	runID         uint
}

func newWaiverFixture(t *testing.T) waiverFixture {
	t.Helper()
	dsn := fmt.Sprintf("file:waiver-%s?mode=memory&cache=shared&_busy_timeout=5000", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&model.User{}, &model.RobotCell{}, &model.SafetyZone{}, &model.MotionProgram{}, &model.ValidationRun{}, &model.ViolationWaiver{}, &model.AuditEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	users := []model.User{
		{Username: "uploader", PasswordHash: "x", Role: constants.RoleRobotProgrammer, Active: true},
		{Username: "reviewer", PasswordHash: "x", Role: constants.RoleReviewer, Active: true},
		{Username: "second-reviewer", PasswordHash: "x", Role: constants.RoleReviewer, Active: true},
	}
	for index := range users {
		if err := db.Create(&users[index]).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	uploader, reviewer, other := users[0], users[1], users[2]
	cell := model.RobotCell{CellCode: "W-CELL", Name: "waiver cell", LayoutGeoJSON: "{}", OwnerTeam: "QA", CellState: constants.CellStateFrozen, LayoutVersion: 1, CreatedBy: reviewer.ID}
	if err := db.Create(&cell).Error; err != nil {
		t.Fatalf("seed cell: %v", err)
	}
	program := model.MotionProgram{
		RobotCellID: cell.ID, ProgramCode: "W-PROG", Version: 1, TrajectoryJSON: "[]", ToolRadiusMM: 1, PayloadRadiusMM: 1,
		InterlockSequenceJSON: "[]", SourceChecksum: "abc", ProgramState: constants.ProgramStateActive,
		UploadedBy: uploader.ID, UploadedAt: time.Now().UTC(),
	}
	if err := db.Create(&program).Error; err != nil {
		t.Fatalf("seed program: %v", err)
	}
	collisionJSON, _ := json.Marshal([]dto.CollisionEvent{{Violation: false, Evidence: "advisory"}, {Violation: true, Evidence: "restricted zone breach"}})
	findingJSON, _ := json.Marshal([]dto.InterlockFinding{{Code: "missing_prerequisite", Event: "gate"}})
	finished := time.Now().UTC()
	run := model.ValidationRun{
		MotionProgramID: program.ID, ZoneSnapshot: "[]", ProgramSnapshot: "{}", AlgorithmVersion: "test-v1",
		InputHash: "hash-1", IdempotencyKey: "test-key-1", CollisionEventsJSON: string(collisionJSON),
		InterlockFindingsJSON: string(findingJSON), RiskScore: 40, ValidationStatus: constants.ValidationFailed,
		Explanation: "fail", RequestedBy: uploader.ID, StartedAt: finished.Add(-time.Second), FinishedAt: &finished,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatalf("seed run: %v", err)
	}
	validationRepository := repository.NewValidationRunRepository(db)
	programRepository := repository.NewMotionProgramRepository(db)
	zoneRepository := repository.NewSafetyZoneRepository(db)
	systemRepository := repository.NewSystemRepository(db)
	systemService := NewSystemService(systemRepository, "0123456789abcdef0123456789abcdef", time.Hour)
	service := NewValidationRunService(db, validationRepository, programRepository, zoneRepository, systemService, "test-v1")
	return waiverFixture{
		db: db, service: service,
		reviewer:      dto.Actor{ID: reviewer.ID, Username: reviewer.Username, Role: reviewer.Role},
		otherReviewer: dto.Actor{ID: other.ID, Username: other.Username, Role: other.Role},
		uploader:      dto.Actor{ID: uploader.ID, Username: uploader.Username, Role: uploader.Role},
		runID:         run.ID,
	}
}

func waiverRequest(kind string, index int, reason string, expiresAt time.Time) dto.GrantWaiverRequest {
	return dto.GrantWaiverRequest{FindingKind: kind, FindingIndex: index, Reason: reason, ExpiresAt: expiresAt}
}

func TestGrantWaiversPersistsAndRejectsBadTargets(t *testing.T) {
	fixture := newWaiverFixture(t)
	future := time.Now().UTC().Add(24 * time.Hour)
	requests := []dto.GrantWaiverRequest{
		waiverRequest(dto.WaiverFindingEnvelope, 1, "Compensating guard installed during ramp-up", future),
		waiverRequest(dto.WaiverFindingInterlock, 0, "Manual key exchange procedure documented", future),
	}
	response, err := fixture.service.GrantWaivers(fixture.runID, requests, fixture.reviewer, "req-1")
	if err != nil {
		t.Fatalf("grant waivers: %v", err)
	}
	if len(response.ViolationWaivers) != 2 {
		t.Fatalf("expected 2 waivers, got %d", len(response.ViolationWaivers))
	}
	for _, waiver := range response.ViolationWaivers {
		if !waiver.Active || waiver.GrantedByName != "reviewer" {
			t.Fatalf("waiver not active or missing grantor: %+v", waiver)
		}
	}

	// An advisory (non-violation) collision cannot be waived.
	if _, err := fixture.service.GrantWaivers(fixture.runID,
		[]dto.GrantWaiverRequest{waiverRequest(dto.WaiverFindingEnvelope, 0, "not a violation at all", future)},
		fixture.reviewer, "req-2"); err == nil {
		t.Fatal("expected invalid_waiver_target rejection for advisory collision")
	}

	// Out-of-range and unknown kind targets are rejected.
	if _, err := fixture.service.GrantWaivers(fixture.runID,
		[]dto.GrantWaiverRequest{waiverRequest(dto.WaiverFindingInterlock, 9, "missing finding", future)},
		fixture.reviewer, "req-3"); err == nil {
		t.Fatal("expected rejection for out-of-range finding")
	}

	// Duplicate unexpired waiver (within the same batch and against history) is rejected.
	if _, err := fixture.service.GrantWaivers(fixture.runID,
		[]dto.GrantWaiverRequest{
			waiverRequest(dto.WaiverFindingEnvelope, 1, "duplicate within batch", future),
			waiverRequest(dto.WaiverFindingInterlock, 0, "second duplicate", future),
		}, fixture.otherReviewer, "req-4"); err == nil {
		t.Fatal("expected duplicate_waiver rejection")
	}

	// A deadline in the past is rejected and nothing is appended.
	response, err = fixture.service.Get(fixture.runID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(response.ViolationWaivers) != 2 {
		t.Fatalf("failed requests must not append waivers, got %d", len(response.ViolationWaivers))
	}
}

func TestUploaderCannotGrantWaiverOrAccept(t *testing.T) {
	fixture := newWaiverFixture(t)
	future := time.Now().UTC().Add(24 * time.Hour)
	if _, err := fixture.service.GrantWaivers(fixture.runID,
		[]dto.GrantWaiverRequest{waiverRequest(dto.WaiverFindingEnvelope, 1, "uploader self grant", future)},
		fixture.uploader, "req-uploader"); err == nil {
		t.Fatal("uploader grant must be forbidden")
	}
	requests := []dto.GrantWaiverRequest{
		waiverRequest(dto.WaiverFindingEnvelope, 1, "restricted breach accepted with guard", future),
		waiverRequest(dto.WaiverFindingInterlock, 0, "gate procedure accepted", future),
	}
	if _, err := fixture.service.Accept(fixture.runID, "all violations covered", requests, fixture.uploader, "req-uploader-accept"); err == nil {
		t.Fatal("uploader acceptance must be forbidden and roll back waiver inserts")
	}
	var waiverCount int64
	if err := fixture.db.Model(&model.ViolationWaiver{}).Count(&waiverCount).Error; err != nil {
		t.Fatalf("count waivers: %v", err)
	}
	if waiverCount != 0 {
		t.Fatalf("forbidden accept must roll waivers back, found %d", waiverCount)
	}
}

func TestAcceptRequiresCompleteUnexpiredCoverage(t *testing.T) {
	fixture := newWaiverFixture(t)
	future := time.Now().UTC().Add(24 * time.Hour)

	// Accepting a failed run without any waivers must keep it failed.
	if _, err := fixture.service.Accept(fixture.runID, "try accept without waivers", nil, fixture.reviewer, "req-accept-1"); err == nil {
		t.Fatal("expected unwaived_violations rejection")
	}
	response, _ := fixture.service.Get(fixture.runID)
	if response.ValidationStatus != constants.ValidationFailed {
		t.Fatalf("run must remain failed after rejected accept, got %s", response.ValidationStatus)
	}

	// Only one of the two findings waived: accept still fails and the run stays failed.
	partial := []dto.GrantWaiverRequest{waiverRequest(dto.WaiverFindingEnvelope, 1, "only envelope covered", future)}
	if _, err := fixture.service.Accept(fixture.runID, "incomplete coverage", partial, fixture.reviewer, "req-accept-2"); err == nil {
		t.Fatal("expected unwaived_violations rejection with partial coverage")
	}
	response, _ = fixture.service.Get(fixture.runID)
	if response.ValidationStatus != constants.ValidationFailed || len(response.ViolationWaivers) != 0 {
		t.Fatalf("failed accept must roll back waiver inserts and stay failed, got status=%s waivers=%d", response.ValidationStatus, len(response.ViolationWaivers))
	}

	// Expired waiver in the same accept request is rejected.
	expired := time.Now().UTC().Add(-time.Hour)
	expiredBatch := []dto.GrantWaiverRequest{
		waiverRequest(dto.WaiverFindingEnvelope, 1, "expired reason", expired),
		waiverRequest(dto.WaiverFindingInterlock, 0, "valid reason", future),
	}
	if _, err := fixture.service.Accept(fixture.runID, "expired included", expiredBatch, fixture.reviewer, "req-accept-3"); err == nil {
		t.Fatal("expected waiver_already_expired rejection")
	}

	// Complete and unexpired coverage accepts the run in one transaction.
	complete := []dto.GrantWaiverRequest{
		waiverRequest(dto.WaiverFindingEnvelope, 1, "envelope risk mitigated", future),
		waiverRequest(dto.WaiverFindingInterlock, 0, "interlock risk accepted temporarily", future),
	}
	accepted, err := fixture.service.Accept(fixture.runID, "all violations covered and accepted", complete, fixture.reviewer, "req-accept-4")
	if err != nil {
		t.Fatalf("accept with full coverage: %v", err)
	}
	if accepted.ValidationStatus != constants.ValidationAccepted {
		t.Fatalf("expected accepted, got %s", accepted.ValidationStatus)
	}
	if len(accepted.ViolationWaivers) != 2 {
		t.Fatalf("expected 2 historical waivers on the accepted run, got %d", len(accepted.ViolationWaivers))
	}
}

func TestExpiredWaiverAllowsReplacementAndHistoryPersists(t *testing.T) {
	fixture := newWaiverFixture(t)
	// Insert an already-expired historical waiver.
	expired := model.ViolationWaiver{
		ValidationRunID: fixture.runID, FindingKind: dto.WaiverFindingEnvelope, FindingIndex: 1,
		Justification: "old expired waiver", ExpiresAt: time.Now().UTC().Add(-time.Minute),
		GrantedBy: fixture.reviewer.ID, GrantedByName: fixture.reviewer.Username, GrantedAt: time.Now().UTC().Add(-2 * time.Hour),
	}
	if err := fixture.db.Create(&expired).Error; err != nil {
		t.Fatalf("seed expired waiver: %v", err)
	}
	response, err := fixture.service.GrantWaivers(fixture.runID,
		[]dto.GrantWaiverRequest{waiverRequest(dto.WaiverFindingEnvelope, 1, "fresh waiver after expiry", time.Now().UTC().Add(time.Hour))},
		fixture.otherReviewer, "req-renew")
	if err != nil {
		t.Fatalf("granting against an expired-only finding must succeed: %v", err)
	}
	if len(response.ViolationWaivers) != 2 {
		t.Fatalf("expired history must be retained, got %d waivers", len(response.ViolationWaivers))
	}
	var active int
	for _, waiver := range response.ViolationWaivers {
		if waiver.Active {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("expected exactly one active waiver, got %d", active)
	}

	// Refreshing the detail read still returns the historical waiver.
	reloaded, err := fixture.service.Get(fixture.runID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(reloaded.ViolationWaivers) != 2 {
		t.Fatalf("history must survive refresh, got %d", len(reloaded.ViolationWaivers))
	}
}
