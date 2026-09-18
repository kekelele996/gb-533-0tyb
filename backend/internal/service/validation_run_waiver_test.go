package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"robot-cell-safety-envelope-validator/backend/internal/constants"
	"robot-cell-safety-envelope-validator/backend/internal/dto"
	"robot-cell-safety-envelope-validator/backend/internal/model"
	"robot-cell-safety-envelope-validator/backend/internal/repository"
)

func newWaiverTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.RobotCell{}, &model.SafetyZone{}, &model.MotionProgram{}, &model.ValidationRun{}, &model.FindingWaiver{}, &model.AuditEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE finding_waivers").Error
		_ = db.Exec("DROP TABLE validation_runs").Error
		_ = db.Exec("DROP TABLE motion_programs").Error
		_ = db.Exec("DROP TABLE robot_cells").Error
		_ = db.Exec("DROP TABLE users").Error
		_ = db.Exec("DROP TABLE audit_events").Error
	})
	return db
}

type waiverFixture struct {
	service  *ValidationRunService
	reviewer dto.Actor
	uploader dto.Actor
	runID    uint
}

func newWaiverFixture(t *testing.T, status string, collisions []dto.CollisionEvent, findings []dto.InterlockFinding) waiverFixture {
	t.Helper()
	db := newWaiverTestDB(t)
	now := time.Now().UTC()
	uploader := model.User{Username: "uploader-user", PasswordHash: "x", Role: constants.RoleRobotProgrammer, Active: true, CreatedAt: now, UpdatedAt: now}
	reviewerUser := model.User{Username: "reviewer-user", PasswordHash: "x", Role: constants.RoleReviewer, Active: true, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&uploader).Error; err != nil {
		t.Fatalf("create uploader: %v", err)
	}
	if err := db.Create(&reviewerUser).Error; err != nil {
		t.Fatalf("create reviewer: %v", err)
	}
	cell := model.RobotCell{CellCode: "CELL-T", Name: "test cell", CellState: constants.CellStateFrozen, LayoutVersion: 1, CreatedBy: uploader.ID}
	if err := db.Create(&cell).Error; err != nil {
		t.Fatalf("create cell: %v", err)
	}
	collisionJSON, _ := json.Marshal(collisions)
	findingJSON, _ := json.Marshal(findings)
	program := model.MotionProgram{
		RobotCellID: cell.ID, ProgramCode: "PRG-T", Version: 1, TrajectoryJSON: "[]", ToolRadiusMM: 10, PayloadRadiusMM: 5,
		InterlockSequenceJSON: "[]", SourceChecksum: strings.Repeat("a", 64), ProgramState: constants.ProgramStateReady,
		UploadedBy: uploader.ID, UploadedAt: now.Add(-time.Hour), UpdatedAt: now,
	}
	if err := db.Create(&program).Error; err != nil {
		t.Fatalf("create program: %v", err)
	}
	finished := now.Add(-time.Minute)
	run := model.ValidationRun{
		MotionProgramID: program.ID, ZoneSnapshot: "[]", ProgramSnapshot: "{}", AlgorithmVersion: "test-v1",
		InputHash: strings.Repeat("b", 64), IdempotencyKey: "idem-" + status + "-" + time.Now().Format(time.RFC3339Nano),
		CollisionEventsJSON: string(collisionJSON), InterlockFindingsJSON: string(findingJSON),
		RiskScore: 50, ValidationStatus: status, Explanation: "fixture", RequestedBy: reviewerUser.ID,
		StartedAt: finished.Add(-time.Second), FinishedAt: &finished,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
	repo := repository.NewValidationRunRepository(db)
	system := NewSystemService(repository.NewSystemRepository(db), strings.Repeat("s", 32), time.Hour)
	service := NewValidationRunService(db, repo, repository.NewMotionProgramRepository(db), repository.NewSafetyZoneRepository(db), system, "test-v1")
	return waiverFixture{
		service: service, runID: run.ID,
		reviewer: dto.Actor{ID: reviewerUser.ID, Username: reviewerUser.Username, Role: constants.RoleReviewer},
		uploader: dto.Actor{ID: uploader.ID, Username: uploader.Username, Role: constants.RoleRobotProgrammer},
	}
}

func waiverViolations() []dto.CollisionEvent {
	return []dto.CollisionEvent{
		{SegmentIndex: 1, FirstTimeMS: 100, ZoneID: 1, ZoneName: "gate", ZoneType: "restricted", Violation: true, Evidence: "enters gate"},
		{SegmentIndex: 2, FirstTimeMS: 200, ZoneID: 2, ZoneName: "aisle", ZoneType: "service", Violation: true, Evidence: "enters aisle"},
		{SegmentIndex: 3, FirstTimeMS: 300, ZoneID: 3, ZoneName: "operating", ZoneType: "operating", Violation: false, Evidence: "advisory contact"},
	}
}

func waiverInterlocks() []dto.InterlockFinding {
	return []dto.InterlockFinding{{Code: "reversed_order", Event: "gate_locked", Evidence: "order reversed"}}
}

func waiverReq(kind string, index int, expiresAt time.Time) dto.FindingWaiverRequest {
	return dto.FindingWaiverRequest{FindingType: kind, FindingIndex: index, Reason: "Remediation scheduled before this deadline.", ExpiresAt: expiresAt}
}

func TestGrantWaiverPersistsAndStaysFailed(t *testing.T) {
	fixture := newWaiverFixture(t, constants.ValidationFailed, waiverViolations(), nil)
	response, err := fixture.service.GrantWaiver(fixture.runID, waiverReq(constants.FindingTypeEnvelopeViolation, 0, time.Now().Add(24*time.Hour)), fixture.reviewer, "req-1")
	if err != nil {
		t.Fatalf("grant waiver: %v", err)
	}
	if response.ValidationStatus != constants.ValidationFailed {
		t.Fatalf("status = %s, want failed (grant alone must not accept)", response.ValidationStatus)
	}
	if len(response.FindingWaivers) != 1 || response.FindingWaivers[0].GrantedByName != "reviewer-user" {
		t.Fatalf("waiver history = %+v", response.FindingWaivers)
	}
}

func TestGrantWaiverRejectsDuplicateActive(t *testing.T) {
	fixture := newWaiverFixture(t, constants.ValidationFailed, waiverViolations(), nil)
	if _, err := fixture.service.GrantWaiver(fixture.runID, waiverReq(constants.FindingTypeEnvelopeViolation, 0, time.Now().Add(24*time.Hour)), fixture.reviewer, "req-1"); err != nil {
		t.Fatalf("first grant: %v", err)
	}
	_, err := fixture.service.GrantWaiver(fixture.runID, waiverReq(constants.FindingTypeEnvelopeViolation, 0, time.Now().Add(48*time.Hour)), fixture.reviewer, "req-2")
	if appErr, ok := err.(*AppError); !ok || appErr.Code != "duplicate_waiver" {
		t.Fatalf("expected duplicate_waiver, got %v", err)
	}
	response, getErr := fixture.service.Get(fixture.runID)
	if getErr != nil || len(response.FindingWaivers) != 1 {
		t.Fatalf("failed duplicate must not append, waivers=%d err=%v", len(response.FindingWaivers), getErr)
	}
}

func TestExpiredWaiverCanBeReplaced(t *testing.T) {
	fixture := newWaiverFixture(t, constants.ValidationFailed, waiverViolations(), nil)
	expired := model.FindingWaiver{
		ValidationRunID: fixture.runID, FindingType: constants.FindingTypeEnvelopeViolation, FindingIndex: 0,
		Reason: "old remediation window", ExpiresAt: time.Now().Add(-time.Hour), GrantedBy: fixture.reviewer.ID,
		GrantedByName: fixture.reviewer.Username, GrantedAt: time.Now().Add(-2 * time.Hour),
	}
	if err := fixture.service.db.Create(&expired).Error; err != nil {
		t.Fatalf("seed expired waiver: %v", err)
	}
	response, err := fixture.service.GrantWaiver(fixture.runID, waiverReq(constants.FindingTypeEnvelopeViolation, 0, time.Now().Add(48*time.Hour)), fixture.reviewer, "req-2")
	if err != nil {
		t.Fatalf("replacement after expiry: %v", err)
	}
	if len(response.FindingWaivers) != 2 {
		t.Fatalf("both historical rows must remain readable, got %d", len(response.FindingWaivers))
	}
}

func TestPastDeadlineRejected(t *testing.T) {
	fixture := newWaiverFixture(t, constants.ValidationFailed, waiverViolations(), nil)
	_, err := fixture.service.GrantWaiver(fixture.runID, waiverReq(constants.FindingTypeEnvelopeViolation, 0, time.Now().Add(-time.Minute)), fixture.reviewer, "req-1")
	if appErr, ok := err.(*AppError); !ok || appErr.Code != "expired_waiver" {
		t.Fatalf("expected expired_waiver, got %v", err)
	}
}

func TestGrantWaiverUploaderForbidden(t *testing.T) {
	fixture := newWaiverFixture(t, constants.ValidationFailed, waiverViolations(), nil)
	_, err := fixture.service.GrantWaiver(fixture.runID, waiverReq(constants.FindingTypeEnvelopeViolation, 0, time.Now().Add(24*time.Hour)), fixture.uploader, "req-1")
	if appErr, ok := err.(*AppError); !ok || appErr.Code != "forbidden" {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestGrantWaiverInvalidFindingRejected(t *testing.T) {
	fixture := newWaiverFixture(t, constants.ValidationFailed, waiverViolations(), nil)
	if _, err := fixture.service.GrantWaiver(fixture.runID, waiverReq(constants.FindingTypeEnvelopeViolation, 5, time.Now().Add(24*time.Hour)), fixture.reviewer, "req-1"); err == nil {
		t.Fatal("out-of-range violation index must be rejected")
	}
	// Informational contacts are not violations and cannot be waived.
	if _, err := fixture.service.GrantWaiver(fixture.runID, waiverReq(constants.FindingTypeEnvelopeViolation, 2, time.Now().Add(24*time.Hour)), fixture.reviewer, "req-2"); err == nil {
		t.Fatal("informational contact must not be waivable")
	}
}

func TestAcceptWithoutCoverageKeepsRunFailed(t *testing.T) {
	fixture := newWaiverFixture(t, constants.ValidationFailed, waiverViolations(), waiverInterlocks())
	_, err := fixture.service.Accept(fixture.runID, "Accepted despite uncovered violations.", nil, fixture.reviewer, "req-1")
	if appErr, ok := err.(*AppError); !ok || appErr.Code != "incomplete_waivers" {
		t.Fatalf("expected incomplete_waivers, got %v", err)
	}
	response, _ := fixture.service.Get(fixture.runID)
	if response.ValidationStatus != constants.ValidationFailed {
		t.Fatalf("status = %s, want failed", response.ValidationStatus)
	}
}

func TestAcceptWaiversAtomicRollbackOnFailure(t *testing.T) {
	fixture := newWaiverFixture(t, constants.ValidationFailed, waiverViolations(), waiverInterlocks())
	// First waiver is valid and new; the second duplicates... nothing, but
	// coverage is also incomplete: interlock finding has no waiver. Reject must
	// roll back even the valid envelope waiver.
	requests := []dto.FindingWaiverRequest{
		waiverReq(constants.FindingTypeEnvelopeViolation, 0, time.Now().Add(24*time.Hour)),
		waiverReq(constants.FindingTypeEnvelopeViolation, 1, time.Now().Add(24*time.Hour)),
	}
	_, err := fixture.service.Accept(fixture.runID, "Attempt atomic accept.", requests, fixture.reviewer, "req-1")
	if appErr, ok := err.(*AppError); !ok || appErr.Code != "incomplete_waivers" {
		t.Fatalf("expected incomplete_waivers, got %v", err)
	}
	response, _ := fixture.service.Get(fixture.runID)
	if response.ValidationStatus != constants.ValidationFailed {
		t.Fatalf("status = %s, want failed", response.ValidationStatus)
	}
	if len(response.FindingWaivers) != 0 {
		t.Fatalf("rolled-back accept must persist no waivers, got %d", len(response.FindingWaivers))
	}
}

func TestAcceptDuplicateWithinRequestRejected(t *testing.T) {
	fixture := newWaiverFixture(t, constants.ValidationFailed, waiverViolations(), nil)
	requests := []dto.FindingWaiverRequest{
		waiverReq(constants.FindingTypeEnvelopeViolation, 0, time.Now().Add(24*time.Hour)),
		waiverReq(constants.FindingTypeEnvelopeViolation, 0, time.Now().Add(48*time.Hour)),
		waiverReq(constants.FindingTypeEnvelopeViolation, 1, time.Now().Add(24*time.Hour)),
	}
	_, err := fixture.service.Accept(fixture.runID, "Duplicate payload.", requests, fixture.reviewer, "req-1")
	if appErr, ok := err.(*AppError); !ok || appErr.Code != "duplicate_waiver" {
		t.Fatalf("expected duplicate_waiver, got %v", err)
	}
	response, _ := fixture.service.Get(fixture.runID)
	if len(response.FindingWaivers) != 0 {
		t.Fatalf("duplicate request must persist nothing, got %d", len(response.FindingWaivers))
	}
}

func TestAcceptConflictsStoredActiveWaiver(t *testing.T) {
	fixture := newWaiverFixture(t, constants.ValidationFailed, waiverViolations(), nil)
	if _, err := fixture.service.GrantWaiver(fixture.runID, waiverReq(constants.FindingTypeEnvelopeViolation, 0, time.Now().Add(24*time.Hour)), fixture.reviewer, "req-1"); err != nil {
		t.Fatalf("grant: %v", err)
	}
	requests := []dto.FindingWaiverRequest{
		waiverReq(constants.FindingTypeEnvelopeViolation, 0, time.Now().Add(48*time.Hour)),
		waiverReq(constants.FindingTypeEnvelopeViolation, 1, time.Now().Add(24*time.Hour)),
	}
	_, err := fixture.service.Accept(fixture.runID, "Conflicts stored waiver.", requests, fixture.reviewer, "req-2")
	if appErr, ok := err.(*AppError); !ok || appErr.Code != "duplicate_waiver" {
		t.Fatalf("expected duplicate_waiver, got %v", err)
	}
}

func TestAcceptSucceedsWithFullCoverage(t *testing.T) {
	fixture := newWaiverFixture(t, constants.ValidationFailed, waiverViolations(), waiverInterlocks())
	requests := []dto.FindingWaiverRequest{
		waiverReq(constants.FindingTypeEnvelopeViolation, 0, time.Now().Add(24*time.Hour)),
		waiverReq(constants.FindingTypeEnvelopeViolation, 1, time.Now().Add(48*time.Hour)),
		waiverReq(constants.FindingTypeInterlockFinding, 0, time.Now().Add(24*time.Hour)),
	}
	response, err := fixture.service.Accept(fixture.runID, "All violations waived with bounded deadlines.", requests, fixture.reviewer, "req-1")
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if response.ValidationStatus != constants.ValidationAccepted {
		t.Fatalf("status = %s, want accepted", response.ValidationStatus)
	}
	if len(response.FindingWaivers) != 3 {
		t.Fatalf("waivers = %d, want 3", len(response.FindingWaivers))
	}
	// History stays readable after a reload (refresh).
	reloaded, err := fixture.service.Get(fixture.runID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(reloaded.FindingWaivers) != 3 {
		t.Fatalf("reloaded waivers = %d, want 3", len(reloaded.FindingWaivers))
	}
	if reloaded.ReviewedBy == nil || *reloaded.ReviewedBy != fixture.reviewer.ID {
		t.Fatal("accepted run must record the accepting reviewer")
	}
}

func TestAcceptUploaderForbidden(t *testing.T) {
	fixture := newWaiverFixture(t, constants.ValidationFailed, waiverViolations(), nil)
	requests := []dto.FindingWaiverRequest{
		waiverReq(constants.FindingTypeEnvelopeViolation, 0, time.Now().Add(24*time.Hour)),
		waiverReq(constants.FindingTypeEnvelopeViolation, 1, time.Now().Add(24*time.Hour)),
	}
	_, err := fixture.service.Accept(fixture.runID, "Uploader self-accept.", requests, fixture.uploader, "req-1")
	if appErr, ok := err.(*AppError); !ok || appErr.Code != "forbidden" {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestExpiredWaiverDoesNotCover(t *testing.T) {
	fixture := newWaiverFixture(t, constants.ValidationFailed, waiverViolations(), nil)
	// Seed an already-expired historical waiver directly, then accept with a
	// waiver only covering the second violation: coverage must fail.
	expired := model.FindingWaiver{
		ValidationRunID: fixture.runID, FindingType: constants.FindingTypeEnvelopeViolation, FindingIndex: 0,
		Reason: "old remediation window", ExpiresAt: time.Now().Add(-time.Hour), GrantedBy: fixture.reviewer.ID,
		GrantedByName: fixture.reviewer.Username, GrantedAt: time.Now().Add(-2 * time.Hour),
	}
	if err := fixture.service.db.Create(&expired).Error; err != nil {
		t.Fatalf("seed expired waiver: %v", err)
	}
	requests := []dto.FindingWaiverRequest{
		waiverReq(constants.FindingTypeEnvelopeViolation, 1, time.Now().Add(24*time.Hour)),
	}
	_, err := fixture.service.Accept(fixture.runID, "Expired history should not count.", requests, fixture.reviewer, "req-1")
	if appErr, ok := err.(*AppError); !ok || appErr.Code != "incomplete_waivers" {
		t.Fatalf("expected incomplete_waivers, got %v", err)
	}
	// After the rejection, the expired row remains in readable history.
	response, _ := fixture.service.Get(fixture.runID)
	if len(response.FindingWaivers) != 1 {
		t.Fatalf("expired history waiver must remain readable, got %d", len(response.FindingWaivers))
	}
}

func TestWaiverLockedAfterTerminalState(t *testing.T) {
	fixture := newWaiverFixture(t, constants.ValidationVoided, waiverViolations(), nil)
	_, err := fixture.service.GrantWaiver(fixture.runID, waiverReq(constants.FindingTypeEnvelopeViolation, 0, time.Now().Add(24*time.Hour)), fixture.reviewer, "req-1")
	if err == nil {
		t.Fatal("voided runs must not accept waivers")
	}
}
