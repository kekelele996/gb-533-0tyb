package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
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

// maxWaiverHorizon bounds how far into the future a reviewer may push a waiver
// deadline. Deadlines are intended for bounded remediation, not permanent
// suppression.
const maxWaiverHorizon = 5 * 365 * 24 * time.Hour

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

// GrantWaiver appends one reviewer exemption to a failed (or already reviewed
// failed) run. The run state never changes; only the waiver history grows.
func (service *ValidationRunService) GrantWaiver(id uint, request dto.FindingWaiverRequest, actor dto.Actor, requestID string) (dto.ValidationRunResponse, error) {
	err := service.db.Transaction(func(tx *gorm.DB) error {
		repo := service.repository.WithDB(tx)
		run, err := repo.GetForUpdate(tx, id)
		if err != nil {
			return MapRepositoryError("validation run", err)
		}
		if err := rejectWaiverTargetState(run.ValidationStatus); err != nil {
			return err
		}
		if run.MotionProgram.UploadedBy == actor.ID {
			return Forbidden("program uploader cannot grant a violation waiver")
		}
		targets, err := violationTargets(run)
		if err != nil {
			return Internal("stored validation evidence is invalid", err)
		}
		normalized, err := normalizeWaiverRequests([]dto.FindingWaiverRequest{request}, targets, run.FindingWaivers, time.Now().UTC())
		if err != nil {
			return err
		}
		waiver := waiverFromRequest(normalized[0], id, actor, time.Now().UTC())
		if err := repo.CreateWaiver(&waiver); err != nil {
			return Internal("could not persist finding waiver", err)
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "validation_run.waiver_granted", "validation_run", auditID(id), map[string]any{
			"finding_type": waiver.FindingType, "finding_index": waiver.FindingIndex,
			"reason_length": len(waiver.Reason), "expires_at": waiver.ExpiresAt,
		}, nil, waiverSummary(waiver))
	})
	if err != nil {
		return dto.ValidationRunResponse{}, err
	}
	return service.Get(id)
}

func (service *ValidationRunService) Accept(id uint, note string, waiverRequests []dto.FindingWaiverRequest, actor dto.Actor, requestID string) (dto.ValidationRunResponse, error) {
	err := service.db.Transaction(func(tx *gorm.DB) error {
		repo := service.repository.WithDB(tx)
		run, err := repo.GetForUpdate(tx, id)
		if err != nil {
			return MapRepositoryError("validation run", err)
		}
		if run.MotionProgram.UploadedBy == actor.ID {
			return Forbidden("program uploader cannot accept their own validation result")
		}
		if run.ValidationStatus != constants.ValidationReviewed && run.ValidationStatus != constants.ValidationFailed {
			return Conflict("invalid_validation_transition", "only reviewed or failed runs can be accepted", repository.ErrStateConflict)
		}
		targets, err := violationTargets(run)
		if err != nil {
			return Internal("stored validation evidence is invalid", err)
		}
		now := time.Now().UTC()
		normalized, err := normalizeWaiverRequests(waiverRequests, targets, run.FindingWaivers, now)
		if err != nil {
			return err
		}
		granted := make([]model.FindingWaiver, 0, len(normalized))
		for _, request := range normalized {
			waiver := waiverFromRequest(request, id, actor, now)
			if err := repo.CreateWaiver(&waiver); err != nil {
				return Internal("could not persist finding waiver", err)
			}
			granted = append(granted, waiver)
		}
		active := activeWaivers(append(copiedWaivers(run.FindingWaivers), granted...), now)
		if missing := missingCoverage(targets, active); len(missing) > 0 {
			return Unprocessable("incomplete_waivers", fmt.Sprintf("acceptance requires an unexpired waiver for every violation; %d of %d uncovered", len(missing), len(targets)), nil)
		}
		from := run.ValidationStatus
		if err := repo.Review(id, from, constants.ValidationAccepted, actor.ID, strings.TrimSpace(note)); err != nil {
			return Conflict("state_conflict", "validation state changed concurrently", err)
		}
		for _, waiver := range granted {
			if err := service.system.RecordAuditTx(tx, actor, requestID, "validation_run.waiver_granted", "validation_run", auditID(id), map[string]any{
				"finding_type": waiver.FindingType, "finding_index": waiver.FindingIndex,
				"reason_length": len(waiver.Reason), "expires_at": waiver.ExpiresAt,
			}, nil, waiverSummary(waiver)); err != nil {
				return err
			}
		}
		return service.system.RecordAuditTx(tx, actor, requestID, "validation_run.accepted", "validation_run", auditID(id), map[string]any{
			"note_length": len(note), "decision_boundary": "offline evidence only",
			"violation_count": len(targets), "waivers_granted": len(granted), "from_status": from,
		}, validationSummary(run), map[string]any{"validation_status": constants.ValidationAccepted, "violation_count": len(targets), "waivers_granted": len(granted)})
	})
	if err != nil {
		return dto.ValidationRunResponse{}, err
	}
	return service.Get(id)
}

func rejectWaiverTargetState(status string) error {
	if status == constants.ValidationFailed || status == constants.ValidationReviewed {
		return nil
	}
	if status == constants.ValidationAccepted || status == constants.ValidationVoided {
		return Conflict("waiver_locked", "accepted or voided runs no longer accept waivers", repository.ErrStateConflict)
	}
	return Conflict("invalid_validation_transition", "waivers can only be attached to completed failed runs", repository.ErrStateConflict)
}

type waiverTargetInfo struct {
	key   string
	label string
}

// violationTargets returns the stable identity of every finding that must be
// waived before a failed run may be accepted: violating envelope contacts plus
// all interlock findings. Informational contacts never fail the run.
func violationTargets(run model.ValidationRun) (map[string]waiverTargetInfo, error) {
	var collisions []dto.CollisionEvent
	if err := json.Unmarshal([]byte(run.CollisionEventsJSON), &collisions); err != nil {
		return nil, err
	}
	var findings []dto.InterlockFinding
	if err := json.Unmarshal([]byte(run.InterlockFindingsJSON), &findings); err != nil {
		return nil, err
	}
	targets := make(map[string]waiverTargetInfo, len(collisions)+len(findings))
	violationIndex := 0
	for _, collision := range collisions {
		if !collision.Violation {
			continue
		}
		key := waiverKey(constants.FindingTypeEnvelopeViolation, violationIndex)
		targets[key] = waiverTargetInfo{key: key, label: fmt.Sprintf("envelope violation #%d (%s at %.0f ms)", violationIndex+1, collision.ZoneName, collision.FirstTimeMS)}
		violationIndex++
	}
	for index, finding := range findings {
		key := waiverKey(constants.FindingTypeInterlockFinding, index)
		targets[key] = waiverTargetInfo{key: key, label: fmt.Sprintf("interlock finding #%d (%s on %s)", index+1, finding.Code, finding.Event)}
	}
	return targets, nil
}

func waiverKey(findingType string, index int) string { return fmt.Sprintf("%s:%d", findingType, index) }

// activeWaivers selects the single effective (unexpired) waiver per finding.
// Expired and superseded rows remain visible in history but do not cover.
func activeWaivers(waivers []model.FindingWaiver, now time.Time) map[string]model.FindingWaiver {
	active := map[string]model.FindingWaiver{}
	for _, waiver := range waivers {
		if !waiver.ExpiresAt.After(now) {
			continue
		}
		key := waiverKey(waiver.FindingType, waiver.FindingIndex)
		if existing, ok := active[key]; !ok || waiver.ExpiresAt.After(existing.ExpiresAt) {
			active[key] = waiver
		}
	}
	return active
}

func copiedWaivers(waivers []model.FindingWaiver) []model.FindingWaiver {
	copied := make([]model.FindingWaiver, len(waivers))
	copy(copied, waivers)
	return copied
}

func missingCoverage(targets map[string]waiverTargetInfo, active map[string]model.FindingWaiver) []string {
	missing := make([]string, 0)
	for key := range targets {
		if _, ok := active[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
}

// normalizeWaiverRequests validates the waivers carried by one request against
// the run's actual violations, the stored waiver history, and the rest of the
// same request. Any duplicate, expired or out-of-range submission rejects the
// whole request so nothing is partially applied.
func normalizeWaiverRequests(requests []dto.FindingWaiverRequest, targets map[string]waiverTargetInfo, stored []model.FindingWaiver, now time.Time) ([]dto.FindingWaiverRequest, error) {
	seen := map[string]bool{}
	storedActive := activeWaivers(stored, now)
	normalized := make([]dto.FindingWaiverRequest, 0, len(requests))
	for _, request := range requests {
		request.Reason = strings.TrimSpace(request.Reason)
		if len(request.Reason) < 8 || len(request.Reason) > 1000 {
			return nil, Unprocessable("invalid_waiver", "waiver reason must contain 8 to 1000 characters", nil)
		}
		expiresAt := request.ExpiresAt.UTC()
		if !expiresAt.After(now) {
			return nil, Unprocessable("expired_waiver", "waiver deadline must be in the future", nil)
		}
		if expiresAt.After(now.Add(maxWaiverHorizon)) {
			return nil, Unprocessable("expired_waiver", "waiver deadline cannot exceed five years", nil)
		}
		if !constants.ValidFindingType(request.FindingType) {
			return nil, Unprocessable("invalid_waiver_finding", "finding_type must be envelope_violation or interlock_finding", nil)
		}
		key := waiverKey(request.FindingType, request.FindingIndex)
		target, ok := targets[key]
		if !ok {
			return nil, Unprocessable("invalid_waiver_finding", "the referenced finding does not exist or is not a waivable violation", nil)
		}
		if _, ok := storedActive[key]; ok {
			return nil, Conflict("duplicate_waiver", fmt.Sprintf("%s already has an unexpired waiver; wait for it to expire before granting another", target.label), nil)
		}
		if seen[key] {
			return nil, Conflict("duplicate_waiver", fmt.Sprintf("%s is listed more than once in this request", target.label), nil)
		}
		seen[key] = true
		request.ExpiresAt = expiresAt
		normalized = append(normalized, request)
	}
	return normalized, nil
}

func waiverFromRequest(request dto.FindingWaiverRequest, runID uint, actor dto.Actor, now time.Time) model.FindingWaiver {
	return model.FindingWaiver{
		ValidationRunID: runID, FindingType: request.FindingType, FindingIndex: request.FindingIndex,
		Reason: request.Reason, ExpiresAt: request.ExpiresAt, GrantedBy: actor.ID,
		GrantedByName: actor.Username, GrantedAt: now,
	}
}

func waiverSummary(waiver model.FindingWaiver) map[string]any {
	return map[string]any{
		"finding_type": waiver.FindingType, "finding_index": waiver.FindingIndex,
		"expires_at": waiver.ExpiresAt, "granted_by": waiver.GrantedByName,
	}
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
		ReviewedAt: run.ReviewedAt, ReviewNote: run.ReviewNote, FindingWaivers: waiverResponses(run.FindingWaivers),
	}, nil
}

func waiverResponses(waivers []model.FindingWaiver) []dto.FindingWaiverResponse {
	responses := make([]dto.FindingWaiverResponse, 0, len(waivers))
	for _, waiver := range waivers {
		responses = append(responses, dto.FindingWaiverResponse{
			ID: waiver.ID, ValidationRunID: waiver.ValidationRunID, FindingType: waiver.FindingType,
			FindingIndex: waiver.FindingIndex, Reason: waiver.Reason, ExpiresAt: waiver.ExpiresAt,
			GrantedBy: waiver.GrantedBy, GrantedByName: waiver.GrantedByName, GrantedAt: waiver.GrantedAt,
		})
	}
	sort.Slice(responses, func(i, j int) bool {
		if responses[i].FindingType != responses[j].FindingType {
			return responses[i].FindingType < responses[j].FindingType
		}
		if responses[i].FindingIndex != responses[j].FindingIndex {
			return responses[i].FindingIndex < responses[j].FindingIndex
		}
		return responses[i].GrantedAt.After(responses[j].GrantedAt)
	})
	return responses
}

func validationSummary(run model.ValidationRun) map[string]any {
	return map[string]any{"validation_status": run.ValidationStatus, "risk_score": run.RiskScore, "algorithm_version": run.AlgorithmVersion, "input_hash": run.InputHash, "attempt": run.Attempt}
}
