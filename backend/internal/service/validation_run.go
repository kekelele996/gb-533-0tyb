package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"

	"robot-cell-safety-envelope-validator/backend/internal/algorithm"
	"robot-cell-safety-envelope-validator/backend/internal/constants"
	"robot-cell-safety-envelope-validator/backend/internal/dto"
	"robot-cell-safety-envelope-validator/backend/internal/geometry"
	"robot-cell-safety-envelope-validator/backend/internal/model"
	"robot-cell-safety-envelope-validator/backend/internal/repository"
)

type ValidationRunService struct {
	db               *gorm.DB
	repository       *repository.ValidationRunRepository
	programs         *repository.MotionProgramRepository
	zones            *repository.SafetyZoneRepository
	system           *SystemService
	algorithmVersion string
}

func NewValidationRunService(db *gorm.DB, repository *repository.ValidationRunRepository, programs *repository.MotionProgramRepository, zones *repository.SafetyZoneRepository, system *SystemService, algorithmVersion string) *ValidationRunService {
	return &ValidationRunService{db: db, repository: repository, programs: programs, zones: zones, system: system, algorithmVersion: algorithmVersion}
}

type zoneSnapshotItem struct {
	ID             uint            `json:"id"`
	Name           string          `json:"name"`
	ZoneType       string          `json:"zone_type"`
	PolygonGeoJSON json.RawMessage `json:"polygon_geojson"`
	MinHeightMM    float64         `json:"min_height_mm"`
	MaxHeightMM    float64         `json:"max_height_mm"`
	SpeedLimitMMS  float64         `json:"speed_limit_mm_s"`
	AccessRule     string          `json:"access_rule"`
	Version        int             `json:"version"`
}

func (service *ValidationRunService) Create(request dto.CreateValidationRunRequest, idempotencyKey string, actor dto.Actor, requestID string) (dto.ValidationRunResponse, bool, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 120 {
		return dto.ValidationRunResponse{}, false, BadRequest("invalid_idempotency_key", "Idempotency-Key must contain 8 to 120 characters")
	}
	if existing, err := service.repository.FindByIdempotencyKey(idempotencyKey); err == nil {
		response, responseErr := validationResponse(existing)
		response.Reused = true
		return response, true, responseErr
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.ValidationRunResponse{}, false, Internal("could not check idempotency key", err)
	}
	program, err := service.programs.Get(request.MotionProgramID)
	if err != nil {
		return dto.ValidationRunResponse{}, false, MapRepositoryError("motion program", err)
	}
	if program.ProgramState != constants.ProgramStateReady && program.ProgramState != constants.ProgramStateActive {
		return dto.ValidationRunResponse{}, false, Conflict("program_not_ready", "only ready or active programs can be validated", repository.ErrStateConflict)
	}
	activeZones, err := service.zones.ActiveForCell(program.RobotCellID)
	if err != nil {
		return dto.ValidationRunResponse{}, false, Internal("could not load active zones", err)
	}
	if len(activeZones) == 0 {
		return dto.ValidationRunResponse{}, false, Unprocessable("active_zones_required", "the robot cell must have at least one active safety zone", nil)
	}
	zoneSnapshot, volumes, err := buildZoneSnapshot(activeZones)
	if err != nil {
		return dto.ValidationRunResponse{}, false, Unprocessable("invalid_zone_snapshot", err.Error(), err)
	}
	programSnapshot, trajectory, interlocks, err := buildProgramSnapshot(program)
	if err != nil {
		return dto.ValidationRunResponse{}, false, Unprocessable("invalid_program_snapshot", err.Error(), err)
	}
	inputHash := validationInputHash(programSnapshot, zoneSnapshot, service.algorithmVersion)
	attempt, retryOfID := 1, (*uint)(nil)
	if previous, err := service.repository.LatestByInput(inputHash, service.algorithmVersion); err == nil {
		if previous.ValidationStatus != constants.ValidationFailed || !request.RetryFailed {
			response, responseErr := validationResponse(previous)
			response.Reused = true
			return response, true, responseErr
		}
		attempt, retryOfID = previous.Attempt+1, &previous.ID
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.ValidationRunResponse{}, false, Internal("could not check prior validation input", err)
	}
	collisions := geometry.EvaluateEnvelope(trajectory, program.ToolRadiusMM+program.PayloadRadiusMM, volumes)
	findings := algorithm.AnalyzeInterlocks(interlocks)
	riskScore, status, explanation := summarizeValidation(collisions, findings)
	collisionJSON, _ := json.Marshal(collisions)
	findingJSON, _ := json.Marshal(findings)
	started, finished := time.Now().UTC(), time.Now().UTC()
	run := model.ValidationRun{
		MotionProgramID: program.ID, ZoneSnapshot: string(zoneSnapshot), ProgramSnapshot: string(programSnapshot),
		AlgorithmVersion: service.algorithmVersion, InputHash: inputHash, IdempotencyKey: idempotencyKey,
		Attempt: attempt, RetryOfID: retryOfID, CollisionEventsJSON: string(collisionJSON),
		InterlockFindingsJSON: string(findingJSON), RiskScore: riskScore, ValidationStatus: constants.ValidationQueued,
		Explanation: explanation, RequestedBy: actor.ID, StartedAt: started,
	}
	err = service.db.Transaction(func(tx *gorm.DB) error {
		repo := service.repository.WithDB(tx)
		if err := repo.Create(&run); err != nil {
			return err
		}
		if err := repo.SetSimulating(run.ID); err != nil {
			return err
		}
		run.ValidationStatus, run.FinishedAt = status, &finished
		if err := repo.Finish(&run); err != nil {
			return err
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "validation_run.completed", "validation_run", auditID(run.ID), map[string]any{
			"algorithm_version": service.algorithmVersion, "input_hash": inputHash, "attempt": attempt,
		}, nil, map[string]any{"status": status, "risk_score": riskScore, "collision_count": len(collisions), "interlock_finding_count": len(findings)})
	})
	if err != nil {
		if repository.IsUniqueViolation(err) {
			if existing, lookupErr := service.repository.FindByIdempotencyKey(idempotencyKey); lookupErr == nil {
				response, responseErr := validationResponse(existing)
				response.Reused = true
				return response, true, responseErr
			}
			return dto.ValidationRunResponse{}, false, Conflict("idempotency_conflict", "idempotency key is already in use", err)
		}
		return dto.ValidationRunResponse{}, false, Internal("could not persist validation run", err)
	}
	response, err := service.Get(run.ID)
	return response, false, err
}

func (service *ValidationRunService) Get(id uint) (dto.ValidationRunResponse, error) {
	run, err := service.repository.Get(id)
	if err != nil {
		return dto.ValidationRunResponse{}, MapRepositoryError("validation run", err)
	}
	return validationResponse(run)
}

func (service *ValidationRunService) List(page, pageSize int, programID uint, status string) ([]dto.ValidationRunResponse, dto.PageMeta, error) {
	runs, total, err := service.repository.List(page, pageSize, programID, status)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list validation runs", err)
	}
	responses := make([]dto.ValidationRunResponse, 0, len(runs))
	for _, run := range runs {
		response, err := validationResponse(run)
		if err != nil {
			return nil, dto.PageMeta{}, Internal("stored validation evidence is invalid", err)
		}
		responses = append(responses, response)
	}
	return responses, PageMeta(page, pageSize, total), nil
}

func (service *ValidationRunService) Review(id uint, note string, actor dto.Actor, requestID string) (dto.ValidationRunResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.ValidationRunResponse{}, MapRepositoryError("validation run", err)
	}
	if before.ValidationStatus != constants.ValidationPassed && before.ValidationStatus != constants.ValidationFailed {
		return dto.ValidationRunResponse{}, Conflict("invalid_validation_transition", "only completed passed or failed runs can be reviewed", repository.ErrStateConflict)
	}
	if err := service.repository.Review(id, before.ValidationStatus, constants.ValidationReviewed, actor.ID, strings.TrimSpace(note)); err != nil {
		return dto.ValidationRunResponse{}, Conflict("state_conflict", "validation state changed concurrently", err)
	}
	after, err := service.repository.Get(id)
	if err != nil {
		return dto.ValidationRunResponse{}, Internal("could not reload validation run", err)
	}
	if err := service.system.RecordAudit(actor, requestID, "validation_run.reviewed", "validation_run", auditID(id), map[string]any{"note_length": len(note)}, validationSummary(before), validationSummary(after)); err != nil {
		return dto.ValidationRunResponse{}, err
	}
	return validationResponse(after)
}

func (service *ValidationRunService) Accept(id uint, note string, actor dto.Actor, requestID string) (dto.ValidationRunResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.ValidationRunResponse{}, MapRepositoryError("validation run", err)
	}
	if before.MotionProgram.UploadedBy == actor.ID {
		return dto.ValidationRunResponse{}, Forbidden("program uploader cannot accept their own validation result")
	}
	if before.ValidationStatus != constants.ValidationReviewed {
		return dto.ValidationRunResponse{}, Conflict("invalid_validation_transition", "only reviewed runs can be accepted", repository.ErrStateConflict)
	}
	if err := service.repository.Review(id, constants.ValidationReviewed, constants.ValidationAccepted, actor.ID, strings.TrimSpace(note)); err != nil {
		return dto.ValidationRunResponse{}, Conflict("state_conflict", "validation state changed concurrently", err)
	}
	after, err := service.repository.Get(id)
	if err != nil {
		return dto.ValidationRunResponse{}, Internal("could not reload validation run", err)
	}
	if err := service.system.RecordAudit(actor, requestID, "validation_run.accepted", "validation_run", auditID(id), map[string]any{"note_length": len(note), "decision_boundary": "offline evidence only"}, validationSummary(before), validationSummary(after)); err != nil {
		return dto.ValidationRunResponse{}, err
	}
	return validationResponse(after)
}

func (service *ValidationRunService) Void(id uint, note string, actor dto.Actor, requestID string) (dto.ValidationRunResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.ValidationRunResponse{}, MapRepositoryError("validation run", err)
	}
	allowed := before.ValidationStatus == constants.ValidationPassed || before.ValidationStatus == constants.ValidationFailed || before.ValidationStatus == constants.ValidationReviewed || before.ValidationStatus == constants.ValidationAccepted
	if !allowed {
		return dto.ValidationRunResponse{}, Conflict("invalid_validation_transition", "this validation run cannot be voided from its current state", repository.ErrStateConflict)
	}
	if err := service.repository.Review(id, before.ValidationStatus, constants.ValidationVoided, actor.ID, strings.TrimSpace(note)); err != nil {
		return dto.ValidationRunResponse{}, Conflict("state_conflict", "validation state changed concurrently", err)
	}
	after, err := service.repository.Get(id)
	if err != nil {
		return dto.ValidationRunResponse{}, Internal("could not reload validation run", err)
	}
	if err := service.system.RecordAudit(actor, requestID, "validation_run.voided", "validation_run", auditID(id), map[string]any{"note_length": len(note)}, validationSummary(before), validationSummary(after)); err != nil {
		return dto.ValidationRunResponse{}, err
	}
	return validationResponse(after)
}

func buildZoneSnapshot(zones []model.SafetyZone) ([]byte, []geometry.ZoneVolume, error) {
	snapshot := make([]zoneSnapshotItem, 0, len(zones))
	volumes := make([]geometry.ZoneVolume, 0, len(zones))
	for _, zone := range zones {
		polygon, err := geometry.ParsePolygon([]byte(zone.PolygonGeoJSON))
		if err != nil {
			return nil, nil, fmt.Errorf("zone %d geometry: %w", zone.ID, err)
		}
		snapshot = append(snapshot, zoneSnapshotItem{
			ID: zone.ID, Name: zone.Name, ZoneType: zone.ZoneType, PolygonGeoJSON: json.RawMessage(zone.PolygonGeoJSON),
			MinHeightMM: zone.MinHeightMM, MaxHeightMM: zone.MaxHeightMM, SpeedLimitMMS: zone.SpeedLimitMMS,
			AccessRule: zone.AccessRule, Version: zone.Version,
		})
		volumes = append(volumes, geometry.ZoneVolume{
			ID: zone.ID, Name: zone.Name, ZoneType: zone.ZoneType, Polygon: polygon,
			MinHeightMM: zone.MinHeightMM, MaxHeightMM: zone.MaxHeightMM, SpeedLimitMMS: zone.SpeedLimitMMS,
		})
	}
	encoded, err := json.Marshal(snapshot)
	return encoded, volumes, err
}

func buildProgramSnapshot(program model.MotionProgram) ([]byte, []dto.TrajectoryPoint, []dto.InterlockEvent, error) {
	var trajectory []dto.TrajectoryPoint
	if err := json.Unmarshal([]byte(program.TrajectoryJSON), &trajectory); err != nil {
		return nil, nil, nil, fmt.Errorf("decode trajectory: %w", err)
	}
	if err := geometry.ValidateTrajectory(trajectory); err != nil {
		return nil, nil, nil, err
	}
	var events []dto.InterlockEvent
	if err := json.Unmarshal([]byte(program.InterlockSequenceJSON), &events); err != nil {
		return nil, nil, nil, fmt.Errorf("decode interlock sequence: %w", err)
	}
	if validationErrors := algorithm.ValidateInterlockEvents(events); len(validationErrors) > 0 {
		return nil, nil, nil, errors.New(strings.Join(validationErrors, "; "))
	}
	snapshot := map[string]any{
		"id": program.ID, "robot_cell_id": program.RobotCellID, "program_code": program.ProgramCode,
		"version": program.Version, "trajectory": trajectory, "tool_radius_mm": program.ToolRadiusMM,
		"payload_radius_mm": program.PayloadRadiusMM, "interlock_sequence": events,
		"source_checksum": program.SourceChecksum, "program_state": program.ProgramState,
	}
	encoded, err := json.Marshal(snapshot)
	return encoded, trajectory, events, err
}

func validationInputHash(programSnapshot, zoneSnapshot []byte, algorithmVersion string) string {
	hash := sha256.New()
	_, _ = hash.Write(programSnapshot)
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(zoneSnapshot)
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(algorithmVersion))
	return hex.EncodeToString(hash.Sum(nil))
}

func summarizeValidation(collisions []dto.CollisionEvent, findings []dto.InterlockFinding) (float64, string, string) {
	violations, advisory := 0, 0
	for _, collision := range collisions {
		if collision.Violation {
			violations++
		} else {
			advisory++
		}
	}
	risk := math.Min(100, float64(violations*22+len(findings)*18+advisory*3))
	status := constants.ValidationPassed
	if violations > 0 || len(findings) > 0 {
		status = constants.ValidationFailed
	}
	explanation := fmt.Sprintf("%d envelope violation(s), %d informational contact(s), and %d interlock finding(s). Result uses a sampled 2D-plus-height approximation and is decision support only; it does not authorize robot operation.", violations, advisory, len(findings))
	return risk, status, explanation
}

func validationResponse(run model.ValidationRun) (dto.ValidationRunResponse, error) {
	var collisions []dto.CollisionEvent
	if err := json.Unmarshal([]byte(run.CollisionEventsJSON), &collisions); err != nil {
		return dto.ValidationRunResponse{}, err
	}
	var findings []dto.InterlockFinding
	if err := json.Unmarshal([]byte(run.InterlockFindingsJSON), &findings); err != nil {
		return dto.ValidationRunResponse{}, err
	}
	return dto.ValidationRunResponse{
		ID: run.ID, MotionProgramID: run.MotionProgramID, ProgramCode: run.MotionProgram.ProgramCode,
		ProgramVersion: run.MotionProgram.Version, ZoneSnapshot: json.RawMessage(run.ZoneSnapshot),
		ProgramSnapshot: json.RawMessage(run.ProgramSnapshot), AlgorithmVersion: run.AlgorithmVersion,
		InputHash: run.InputHash, IdempotencyKey: run.IdempotencyKey, Attempt: run.Attempt, RetryOfID: run.RetryOfID,
		CollisionEvents: collisions, InterlockFindings: findings, RiskScore: run.RiskScore,
		ValidationStatus: run.ValidationStatus, Explanation: run.Explanation, RequestedBy: run.RequestedBy,
		StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, ReviewedBy: run.ReviewedBy,
		ReviewedAt: run.ReviewedAt, ReviewNote: run.ReviewNote,
	}, nil
}

func validationSummary(run model.ValidationRun) map[string]any {
	return map[string]any{"validation_status": run.ValidationStatus, "risk_score": run.RiskScore, "algorithm_version": run.AlgorithmVersion, "input_hash": run.InputHash, "attempt": run.Attempt}
}
