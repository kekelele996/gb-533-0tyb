package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"robot-cell-safety-envelope-validator/backend/internal/constants"
	"robot-cell-safety-envelope-validator/backend/internal/dto"
	"robot-cell-safety-envelope-validator/backend/internal/model"
)

type Config struct {
	Port               string
	DBDriver           string
	DBDSN              string
	DBAutoMigrate      bool
	JWTSecret          string
	JWTTTL             time.Duration
	CORSOrigin         string
	LogLevel           string
	RateLimitPerMinute int
	AlgorithmVersion   string
}

func Load() (Config, error) {
	cfg := Config{
		Port: envString("PORT", "8080"), DBDriver: envString("DB_DRIVER", "postgres"),
		DBDSN:         envString("DB_DSN", "host=localhost user=robot_safety password=robot_safety_local_533 dbname=robot_safety port=57533 sslmode=disable TimeZone=UTC"),
		DBAutoMigrate: envBool("DB_AUTO_MIGRATE", true), JWTSecret: envString("JWT_SECRET", ""),
		JWTTTL:     time.Duration(envInt("JWT_TTL_MINUTES", 480)) * time.Minute,
		CORSOrigin: envString("CORS_ORIGIN", "http://localhost:18533"), LogLevel: envString("LOG_LEVEL", "info"),
		RateLimitPerMinute: envInt("RATE_LIMIT_PER_MINUTE", 300), AlgorithmVersion: envString("ALGORITHM_VERSION", "envelope-2d-height-v1.0"),
	}
	if len(cfg.JWTSecret) < 24 {
		return Config{}, errors.New("JWT_SECRET must contain at least 24 bytes")
	}
	if cfg.RateLimitPerMinute < 30 || cfg.RateLimitPerMinute > 10000 {
		return Config{}, errors.New("RATE_LIMIT_PER_MINUTE must be between 30 and 10000")
	}
	if cfg.JWTTTL < 5*time.Minute || cfg.JWTTTL > 24*time.Hour {
		return Config{}, errors.New("JWT_TTL_MINUTES must be between 5 and 1440")
	}
	if cfg.AlgorithmVersion == "" {
		return Config{}, errors.New("ALGORITHM_VERSION is required")
	}
	return cfg, nil
}

func OpenDatabase(cfg Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch cfg.DBDriver {
	case "postgres":
		dialector = postgres.Open(cfg.DBDSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.DBDSN)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q", cfg.DBDriver)
	}
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if cfg.DBAutoMigrate {
		if err := db.AutoMigrate(
			&model.User{}, &model.RobotCell{}, &model.SafetyZone{}, &model.MotionProgram{},
			&model.ValidationRun{}, &model.AuditEvent{},
		); err != nil {
			return nil, fmt.Errorf("migrate database: %w", err)
		}
	}
	if err := seed(db, cfg); err != nil {
		return nil, fmt.Errorf("seed database: %w", err)
	}
	return db, nil
}

func seed(db *gorm.DB, cfg Config) error {
	accounts := []struct{ username, password, role string }{
		{"admin", "Admin#533", constants.RoleAdmin},
		{"programmer", "Program#533", constants.RoleRobotProgrammer},
		{"engineer", "Safety#533", constants.RoleSafetyEngineer},
		{"reviewer", "Review#533", constants.RoleReviewer},
		{"auditor", "Audit#533", constants.RoleAuditor},
	}
	for _, account := range accounts {
		var count int64
		if err := db.Model(&model.User{}).Where("username = ?", account.username).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(account.password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := db.Create(&model.User{Username: account.username, PasswordHash: string(hash), Role: account.role, Active: true}).Error; err != nil {
			return err
		}
	}
	var cellCount int64
	if err := db.Model(&model.RobotCell{}).Count(&cellCount).Error; err != nil || cellCount > 0 {
		return err
	}
	var engineer, programmer, reviewer model.User
	if err := db.Where("username = ?", "engineer").First(&engineer).Error; err != nil {
		return err
	}
	if err := db.Where("username = ?", "programmer").First(&programmer).Error; err != nil {
		return err
	}
	if err := db.Where("username = ?", "reviewer").First(&reviewer).Error; err != nil {
		return err
	}
	layout := `{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"kind":"cell_boundary"},"geometry":{"type":"Polygon","coordinates":[[[-2400,-1800],[2400,-1800],[2400,1800],[-2400,1800],[-2400,-1800]]]}}]}`
	cell := model.RobotCell{
		CellCode: "CELL-A17", Name: "Final assembly guarded cell", LayoutGeoJSON: layout,
		RobotModel: "KUKA KR 210 R2700", ControllerModel: "KRC4", MaxReachMM: 2700,
		OwnerTeam: "Assembly Integration", CellState: constants.CellStateFrozen, LayoutVersion: 3, CreatedBy: engineer.ID,
	}
	if err := db.Create(&cell).Error; err != nil {
		return err
	}
	zones := []model.SafetyZone{
		{
			RobotCellID: cell.ID, Name: "Nominal operating envelope", ZoneType: constants.ZoneTypeOperating,
			PolygonGeoJSON: `{"type":"Polygon","coordinates":[[[-900,-900],[900,-900],[900,900],[-900,900],[-900,-900]]]}`,
			MinHeightMM:    0, MaxHeightMM: 2200, SpeedLimitMMS: 1500, AccessRule: "Automatic motion only while all guards are closed",
			ZoneState: constants.ZoneStateActive, Version: 2, CreatedBy: engineer.ID,
		},
		{
			RobotCellID: cell.ID, Name: "Operator transfer gate", ZoneType: constants.ZoneTypeRestricted,
			PolygonGeoJSON: `{"type":"Polygon","coordinates":[[[1050,-500],[1750,-500],[1750,500],[1050,500],[1050,-500]]]}`,
			MinHeightMM:    0, MaxHeightMM: 2400, SpeedLimitMMS: 250, AccessRule: "Gate lock and light curtain clear must precede motion",
			ZoneState: constants.ZoneStateActive, Version: 4, CreatedBy: engineer.ID,
		},
		{
			RobotCellID: cell.ID, Name: "Rear maintenance aisle", ZoneType: constants.ZoneTypeService,
			PolygonGeoJSON: `{"type":"Polygon","coordinates":[[[-1900,1050],[1900,1050],[1900,1550],[-1900,1550],[-1900,1050]]]}`,
			MinHeightMM:    0, MaxHeightMM: 2600, SpeedLimitMMS: 0, AccessRule: "Lockout and maintenance key exchange required",
			ZoneState: constants.ZoneStateActive, Version: 1, CreatedBy: engineer.ID,
		},
	}
	if err := db.Create(&zones).Error; err != nil {
		return err
	}
	trajectory := []dto.TrajectoryPoint{
		{XMM: -650, YMM: 0, ZMM: 750, TimeMS: 0, SpeedMMS: 420},
		{XMM: 300, YMM: 120, ZMM: 820, TimeMS: 2200, SpeedMMS: 460},
		{XMM: 1280, YMM: 80, ZMM: 900, TimeMS: 4300, SpeedMMS: 470},
	}
	interlocks := []dto.InterlockEvent{
		{Name: "emergency_stop_reset", Sequence: 1, DependsOn: []string{}},
		{Name: "transfer_gate_locked", Sequence: 2, DependsOn: []string{"emergency_stop_reset"}},
		{Name: "light_curtain_clear", Sequence: 3, DependsOn: []string{"transfer_gate_locked"}},
		{Name: "automatic_speed_selected", Sequence: 4, DependsOn: []string{"light_curtain_clear"}},
	}
	trajectoryJSON, _ := json.Marshal(trajectory)
	interlockJSON, _ := json.Marshal(interlocks)
	program := model.MotionProgram{
		RobotCellID: cell.ID, ProgramCode: "WELD-SEAM-07", Version: 3, TrajectoryJSON: string(trajectoryJSON),
		ToolRadiusMM: 180, PayloadRadiusMM: 120, InterlockSequenceJSON: string(interlockJSON),
		SourceChecksum: "ddb2b8a02d67301d5165ec0553b75f0ef9f174b074af50b98df21e11b8a3533f",
		ProgramState:   constants.ProgramStateActive, UploadedBy: programmer.ID, UploadedAt: time.Now().UTC().Add(-36 * time.Hour),
	}
	if err := db.Create(&program).Error; err != nil {
		return err
	}
	zoneSnapshot, _ := json.Marshal([]map[string]any{
		{"id": zones[0].ID, "name": zones[0].Name, "zone_type": zones[0].ZoneType, "polygon_geojson": json.RawMessage(zones[0].PolygonGeoJSON), "min_height_mm": zones[0].MinHeightMM, "max_height_mm": zones[0].MaxHeightMM, "speed_limit_mm_s": zones[0].SpeedLimitMMS, "access_rule": zones[0].AccessRule, "version": zones[0].Version},
		{"id": zones[1].ID, "name": zones[1].Name, "zone_type": zones[1].ZoneType, "polygon_geojson": json.RawMessage(zones[1].PolygonGeoJSON), "min_height_mm": zones[1].MinHeightMM, "max_height_mm": zones[1].MaxHeightMM, "speed_limit_mm_s": zones[1].SpeedLimitMMS, "access_rule": zones[1].AccessRule, "version": zones[1].Version},
	})
	programSnapshot, _ := json.Marshal(map[string]any{
		"id": program.ID, "robot_cell_id": program.RobotCellID, "program_code": program.ProgramCode,
		"version": program.Version, "trajectory": trajectory, "tool_radius_mm": program.ToolRadiusMM,
		"payload_radius_mm": program.PayloadRadiusMM, "interlock_sequence": interlocks,
		"source_checksum": program.SourceChecksum, "program_state": program.ProgramState,
	})
	collisions, _ := json.Marshal([]dto.CollisionEvent{{
		SegmentIndex: 1, FirstTimeMS: 3560, ZoneID: zones[1].ID, ZoneName: zones[1].Name,
		ZoneType: zones[1].ZoneType, AllowedSpeedMMS: 250, ActualSpeedMMS: 470, ClearanceMM: -120,
		Violation: true, Evidence: "expanded envelope intersects restricted transfer gate and exceeds its speed limit",
	}})
	findings, _ := json.Marshal([]dto.InterlockFinding{})
	finished := time.Now().UTC().Add(-30 * time.Hour)
	reviewerID := reviewer.ID
	reviewedAt := finished.Add(2 * time.Hour)
	run := model.ValidationRun{
		MotionProgramID: program.ID, ZoneSnapshot: string(zoneSnapshot), ProgramSnapshot: string(programSnapshot),
		AlgorithmVersion: cfg.AlgorithmVersion, InputHash: "b6391a7f33bc8cb8681d56d1d7899593c2be1c1b0927a06db604dd4644e893a0",
		IdempotencyKey: "seed-validation-cell-a17-v3", Attempt: 1, CollisionEventsJSON: string(collisions),
		InterlockFindingsJSON: string(findings), RiskScore: 44, ValidationStatus: constants.ValidationReviewed,
		Explanation: "One restricted-zone envelope and speed violation. Offline approximation only; not an authorization to operate.",
		RequestedBy: engineer.ID, StartedAt: finished.Add(-2 * time.Second), FinishedAt: &finished,
		ReviewedBy: &reviewerID, ReviewedAt: &reviewedAt, ReviewNote: "Guard geometry requires integrator revision before independent acceptance.",
	}
	return db.Create(&run).Error
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
