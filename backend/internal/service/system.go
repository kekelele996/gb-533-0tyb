package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"robot-cell-safety-envelope-validator/backend/internal/dto"
	"robot-cell-safety-envelope-validator/backend/internal/model"
	"robot-cell-safety-envelope-validator/backend/internal/repository"
)

type AppError struct {
	Status  int
	Code    string
	Message string
	Cause   error
}

func (err *AppError) Error() string {
	if err.Cause == nil {
		return err.Message
	}
	return fmt.Sprintf("%s: %v", err.Message, err.Cause)
}
func (err *AppError) Unwrap() error { return err.Cause }

func BadRequest(code, message string, causes ...error) *AppError {
	return &AppError{Status: http.StatusBadRequest, Code: code, Message: message, Cause: firstCause(causes)}
}
func Unprocessable(code, message string, cause error) *AppError {
	return &AppError{Status: http.StatusUnprocessableEntity, Code: code, Message: message, Cause: cause}
}
func Unauthorized(message string) *AppError {
	return &AppError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: message}
}
func Forbidden(message string) *AppError {
	return &AppError{Status: http.StatusForbidden, Code: "forbidden", Message: message}
}
func NotFound(resource string, cause error) *AppError {
	return &AppError{Status: http.StatusNotFound, Code: "not_found", Message: resource + " was not found", Cause: cause}
}
func Conflict(code, message string, cause error) *AppError {
	return &AppError{Status: http.StatusConflict, Code: code, Message: message, Cause: cause}
}
func Internal(message string, cause error) *AppError {
	return &AppError{Status: http.StatusInternalServerError, Code: "internal_error", Message: message, Cause: cause}
}
func firstCause(causes []error) error {
	if len(causes) == 0 {
		return nil
	}
	return causes[0]
}

func MapRepositoryError(resource string, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(strings.ToLower(err.Error()), "record not found") {
		return NotFound(resource, err)
	}
	return Internal("database operation failed", err)
}

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type SystemService struct {
	repository *repository.SystemRepository
	secret     []byte
	ttl        time.Duration
}

func NewSystemService(repository *repository.SystemRepository, secret string, ttl time.Duration) *SystemService {
	return &SystemService{repository: repository, secret: []byte(secret), ttl: ttl}
}

func (service *SystemService) Login(request dto.LoginRequest) (dto.LoginResponse, error) {
	user, err := service.repository.FindUser(strings.TrimSpace(request.Username))
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(request.Password)) != nil {
		return dto.LoginResponse{}, Unauthorized("invalid username or password")
	}
	now := time.Now().UTC()
	expiresAt := now.Add(service.ttl)
	claims := Claims{
		UserID: user.ID, Username: user.Username, Role: user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: fmt.Sprintf("%d", user.ID), Issuer: "robot-safety-api",
			IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(service.secret)
	if err != nil {
		return dto.LoginResponse{}, Internal("could not issue access token", err)
	}
	return dto.LoginResponse{Token: token, ExpiresAt: expiresAt, User: dto.UserView{ID: user.ID, Username: user.Username, Role: user.Role}}, nil
}

func (service *SystemService) ParseToken(encoded string) (dto.Actor, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(encoded, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return service.secret, nil
	}, jwt.WithIssuer("robot-safety-api"), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return dto.Actor{}, Unauthorized("access token is invalid or expired")
	}
	return dto.Actor{ID: claims.UserID, Username: claims.Username, Role: claims.Role}, nil
}

func (service *SystemService) RecordAudit(actor dto.Actor, requestID, action, resourceType, resourceID string, parameters, before, after any) error {
	return service.record(service.repository, actor, requestID, action, resourceType, resourceID, parameters, before, after)
}

func (service *SystemService) RecordAuditTx(db *gorm.DB, actor dto.Actor, requestID, action, resourceType, resourceID string, parameters, before, after any) error {
	return service.record(service.repository.WithDB(db), actor, requestID, action, resourceType, resourceID, parameters, before, after)
}

func (service *SystemService) record(repo *repository.SystemRepository, actor dto.Actor, requestID, action, resourceType, resourceID string, parameters, before, after any) error {
	event := model.AuditEvent{
		ActorID: actor.ID, Actor: actor.Username, Role: actor.Role, Action: action,
		ResourceType: resourceType, ResourceID: resourceID, RequestID: requestID,
		ParametersJSON: encodeSummary(parameters), BeforeJSON: encodeSummary(before), AfterJSON: encodeSummary(after),
	}
	if err := repo.CreateAudit(&event); err != nil {
		return Internal("could not record audit event", err)
	}
	return nil
}

func (service *SystemService) ListAudit(page, pageSize int, actor, requestID, resourceType, action string, from, to *time.Time) ([]dto.AuditEventResponse, dto.PageMeta, error) {
	events, total, err := service.repository.ListAudit(page, pageSize, actor, requestID, resourceType, action, from, to)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list audit events", err)
	}
	responses := make([]dto.AuditEventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, dto.AuditEventResponse{
			ID: event.ID, Actor: event.Actor, Role: event.Role, Action: event.Action,
			ResourceType: event.ResourceType, ResourceID: event.ResourceID, RequestID: event.RequestID,
			Parameters: decodeSummary(event.ParametersJSON), Before: decodeSummary(event.BeforeJSON),
			After: decodeSummary(event.AfterJSON), CreatedAt: event.CreatedAt,
		})
	}
	return responses, PageMeta(page, pageSize, total), nil
}

func PageMeta(page, pageSize int, total int64) dto.PageMeta {
	return dto.PageMeta{Page: page, PageSize: pageSize, Total: total, TotalPages: int(math.Ceil(float64(total) / float64(pageSize)))}
}

func encodeSummary(value any) string {
	if value == nil {
		return "{}"
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return `{"summary":"unavailable"}`
	}
	return string(encoded)
}

func decodeSummary(encoded string) map[string]interface{} {
	result := map[string]interface{}{}
	if encoded == "" {
		return result
	}
	if err := json.Unmarshal([]byte(encoded), &result); err != nil {
		return map[string]interface{}{"raw": encoded}
	}
	return result
}

func auditID(id uint) string { return fmt.Sprintf("%d", id) }
