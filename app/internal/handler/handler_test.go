package handler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"booking-service/internal/api"
	"booking-service/internal/handler"
	"booking-service/internal/mocks"
	"booking-service/internal/models"
	"booking-service/internal/service"
	"booking-service/pkg/logger"

	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"
)

func TestHandler_PostBookingsCreate(t *testing.T) {
	type fields struct {
		bookingService *mocks.MockBookingService
	}

	userID := uuid.New()
	slotID := uuid.New()
	bookingID := uuid.New()
	trueVal := true

	tests := []struct {
		name    string
		prepare func(f fields)
		ctx     context.Context
		request api.PostBookingsCreateRequestObject
		want    api.PostBookingsCreateResponseObject
	}{
		{
			name: "Success_Created",
			ctx:  setUserInContext(context.Background(), userID.String(), models.RoleUser),
			request: api.PostBookingsCreateRequestObject{
				Body: &api.PostBookingsCreateJSONRequestBody{
					SlotId:               slotID,
					CreateConferenceLink: &trueVal,
				},
			},
			prepare: func(f fields) {
				f.bookingService.EXPECT().
					Create(gomock.Any(), userID, slotID).
					Return(&models.Booking{ID: bookingID}, nil)
			},
			want: api.PostBookingsCreate201JSONResponse{
				Booking: &api.Booking{Id: bookingID},
			},
		},
		{
			name: "Failure_Unauthorized",
			ctx:  context.Background(),
			request: api.PostBookingsCreateRequestObject{
				Body: &api.PostBookingsCreateJSONRequestBody{SlotId: slotID},
			},
			prepare: func(f fields) {},
			want: api.PostBookingsCreate401JSONResponse{
				Error: handler.MakeError(api.UNAUTHORIZED, handler.MsgUnauthorized),
			},
		},
		{
			name: "Failure_Forbidden_Role",
			ctx:  setUserInContext(context.Background(), userID.String(), "admin"),
			request: api.PostBookingsCreateRequestObject{
				Body: &api.PostBookingsCreateJSONRequestBody{SlotId: slotID},
			},
			prepare: func(f fields) {},
			want: api.PostBookingsCreate403JSONResponse{
				Error: handler.MakeError(api.FORBIDDEN, handler.MsgForbiddenOnlyUsers),
			},
		},
		{
			name: "Failure_Invalid_UserID_Format",
			ctx:  setUserInContext(context.Background(), "invalid-uuid", models.RoleUser),
			request: api.PostBookingsCreateRequestObject{
				Body: &api.PostBookingsCreateJSONRequestBody{SlotId: slotID},
			},
			prepare: func(f fields) {},
			want: api.PostBookingsCreate400JSONResponse{
				Error: handler.MakeError(api.INVALIDREQUEST, handler.MsgInvalidUserID),
			},
		},
		{
			name: "Failure_Slot_NotFound",
			ctx:  setUserInContext(context.Background(), userID.String(), models.RoleUser),
			request: api.PostBookingsCreateRequestObject{
				Body: &api.PostBookingsCreateJSONRequestBody{SlotId: slotID},
			},
			prepare: func(f fields) {
				f.bookingService.EXPECT().
					Create(gomock.Any(), userID, slotID).
					Return(nil, service.ErrSlotNotExisting)
			},
			want: api.PostBookingsCreate404JSONResponse{
				Error: handler.MakeError(api.NOTFOUND, handler.MsgSlotNotFound),
			},
		},
		{
			name: "Failure_Slot_Already_Booked",
			ctx:  setUserInContext(context.Background(), userID.String(), models.RoleUser),
			request: api.PostBookingsCreateRequestObject{
				Body: &api.PostBookingsCreateJSONRequestBody{SlotId: slotID},
			},
			prepare: func(f fields) {
				f.bookingService.EXPECT().
					Create(gomock.Any(), userID, slotID).
					Return(nil, service.ErrSlotIsUsed)
			},
			want: api.PostBookingsCreate409JSONResponse{
				Error: handler.MakeError(api.SLOTALREADYBOOKED, handler.MsgSlotAlreadyBooked),
			},
		},
		{
			name: "Failure_Internal_Error",
			ctx:  setUserInContext(context.Background(), userID.String(), models.RoleUser),
			request: api.PostBookingsCreateRequestObject{
				Body: &api.PostBookingsCreateJSONRequestBody{SlotId: slotID},
			},
			prepare: func(f fields) {
				f.bookingService.EXPECT().
					Create(gomock.Any(), userID, slotID).
					Return(nil, errors.New("db crash"))
			},
			want: api.PostBookingsCreate500JSONResponse{
				Error: handler.MakeInternalError(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := fields{
				bookingService: mocks.NewMockBookingService(ctrl),
			}
			tt.prepare(f)

			h := handler.NewHandler(f.bookingService, nil, nil, nil, 10, 50, 1, 1, 1)

			testLog := zaptest.NewLogger(t)
			ctx := logger.ToContext(tt.ctx, testLog)

			got, err := h.PostBookingsCreate(ctx, tt.request)

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandler_GetBookingsList(t *testing.T) {
	type fields struct {
		bookingService *mocks.MockBookingService
	}

	const (
		defPage     = uint64(1)
		minPage     = uint64(1)
		defPageSize = uint64(10)
		maxPageSize = uint64(50)
		minPageSize = uint64(1)
	)

	adminID := uuid.New().String()

	ptrInt := func(i int) *int { return &i }

	tests := []struct {
		name    string
		prepare func(f fields)
		ctx     context.Context
		request api.GetBookingsListRequestObject
		want    api.GetBookingsListResponseObject
	}{
		{
			name: "Success_FullList",
			ctx:  setUserInContext(context.Background(), adminID, models.RoleAdmin),
			request: api.GetBookingsListRequestObject{
				Params: api.GetBookingsListParams{
					Page:     ptrInt(2),
					PageSize: ptrInt(20),
				},
			},
			prepare: func(f fields) {
				f.bookingService.EXPECT().
					GetList(gomock.Any(), uint64(20), uint64(1)).
					Return(&models.BookingList{
						Bookings: []models.Booking{{ID: uuid.New()}},
						Total:    100,
					}, nil)
			},
			want: api.GetBookingsList200JSONResponse{
				Bookings: &[]api.Booking{{Id: uuid.New()}},
				Pagination: &api.Pagination{
					Page:     2,
					PageSize: 20,
					Total:    100,
				},
			},
		},
		{
			name: "Success_DefaultPagination",
			ctx:  setUserInContext(context.Background(), adminID, models.RoleAdmin),
			request: api.GetBookingsListRequestObject{
				Params: api.GetBookingsListParams{},
			},
			prepare: func(f fields) {
				f.bookingService.EXPECT().
					GetList(gomock.Any(), defPageSize, defPage-1).
					Return(&models.BookingList{Bookings: []models.Booking{}, Total: 0}, nil)
			},
			want: api.GetBookingsList200JSONResponse{
				Bookings: &[]api.Booking{},
				Pagination: &api.Pagination{
					Page:     int(defPage),
					PageSize: int(defPageSize),
					Total:    0,
				},
			},
		},
		{
			name: "Failure_Unauthorized",
			ctx:  context.Background(),
			want: api.GetBookingsList401JSONResponse{
				Error: handler.MakeError(api.UNAUTHORIZED, handler.MsgUnauthorized),
			},
		},
		{
			name: "Failure_Forbidden_NotAdmin",
			ctx:  setUserInContext(context.Background(), adminID, models.RoleUser),
			want: api.GetBookingsList403JSONResponse{
				Error: handler.MakeError(api.FORBIDDEN, handler.MsgForbiddenOnlyAdmins),
			},
		},
		{
			name: "Failure_InvalidPage_TooSmall",
			ctx:  setUserInContext(context.Background(), adminID, models.RoleAdmin),
			request: api.GetBookingsListRequestObject{
				Params: api.GetBookingsListParams{Page: ptrInt(0)}, // minPage = 1
			},
			want: api.GetBookingsList400JSONResponse{
				Error: handler.MakeError(api.INVALIDREQUEST, handler.MsgInvalidPage),
			},
		},
		{
			name: "Failure_InvalidPageSize_TooLarge",
			ctx:  setUserInContext(context.Background(), adminID, models.RoleAdmin),
			request: api.GetBookingsListRequestObject{
				Params: api.GetBookingsListParams{PageSize: ptrInt(100)},
			},
			want: api.GetBookingsList400JSONResponse{
				Error: handler.MakeError(api.INVALIDREQUEST, handler.MsgInvalidPageSize),
			},
		},
		{
			name: "Failure_InternalServerError",
			ctx:  setUserInContext(context.Background(), adminID, models.RoleAdmin),
			request: api.GetBookingsListRequestObject{
				Params: api.GetBookingsListParams{},
			},
			prepare: func(f fields) {
				f.bookingService.EXPECT().
					GetList(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("db error"))
			},
			want: api.GetBookingsList500JSONResponse{
				Error: handler.MakeInternalError(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := fields{
				bookingService: mocks.NewMockBookingService(ctrl),
			}
			if tt.prepare != nil {
				tt.prepare(f)
			}

			h := handler.NewHandler(
				f.bookingService, nil, nil, nil,
				defPageSize, maxPageSize, minPageSize, defPage, minPage,
			)

			ctx := logger.ToContext(tt.ctx, zaptest.NewLogger(t))
			got, err := h.GetBookingsList(ctx, tt.request)

			assert.NoError(t, err)

			if tt.name == "Success_FullList" {
				res := got.(api.GetBookingsList200JSONResponse)
				assert.Equal(t, tt.want.(api.GetBookingsList200JSONResponse).Pagination, res.Pagination)
				assert.Len(t, *res.Bookings, 1)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestHandler_GetBookingsMy(t *testing.T) {
	type fields struct {
		bookingService *mocks.MockBookingService
	}

	userID := uuid.New()
	bookingID := uuid.New()

	tests := []struct {
		name    string
		prepare func(f fields)
		ctx     context.Context
		request api.GetBookingsMyRequestObject
		want    api.GetBookingsMyResponseObject
	}{
		{
			name:    "Success_GetMyBookings",
			ctx:     setUserInContext(context.Background(), userID.String(), models.RoleUser),
			request: api.GetBookingsMyRequestObject{},
			prepare: func(f fields) {
				f.bookingService.EXPECT().
					GetByUserID(gomock.Any(), userID).
					Return([]models.Booking{
						{ID: bookingID, UserID: userID},
					}, nil)
			},
			want: api.GetBookingsMy200JSONResponse{
				Bookings: &[]api.Booking{{Id: bookingID, UserId: userID}},
			},
		},
		{
			name:    "Failure_Unauthorized",
			ctx:     context.Background(),
			request: api.GetBookingsMyRequestObject{},
			prepare: func(f fields) {},
			want: api.GetBookingsMy401JSONResponse{
				// Используем константы из твоего пакета handler
				Error: handler.MakeError(api.UNAUTHORIZED, handler.MsgUnauthorized),
			},
		},
		{
			name:    "Failure_Invalid_UUID_Format",
			ctx:     setUserInContext(context.Background(), "invalid-uuid", models.RoleUser),
			request: api.GetBookingsMyRequestObject{},
			prepare: func(f fields) {},
			want: api.GetBookingsMy403JSONResponse{
				Error: handler.MakeError(api.INVALIDREQUEST, handler.MsgInvalidUserID),
			},
		},
		{
			name:    "Failure_Service_Error",
			ctx:     setUserInContext(context.Background(), userID.String(), models.RoleUser),
			request: api.GetBookingsMyRequestObject{},
			prepare: func(f fields) {
				f.bookingService.EXPECT().
					GetByUserID(gomock.Any(), userID).
					Return(nil, errors.New("internal service error"))
			},
			want: api.GetBookingsMy500JSONResponse{
				Error: handler.MakeInternalError(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := fields{
				bookingService: mocks.NewMockBookingService(ctrl),
			}
			tt.prepare(f)

			h := handler.NewHandler(f.bookingService, nil, nil, nil, 10, 50, 1, 1, 1)

			ctx := logger.ToContext(tt.ctx, zaptest.NewLogger(t))

			got, err := h.GetBookingsMy(ctx, tt.request)

			assert.NoError(t, err)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandler_PostBookingsBookingIdCancel(t *testing.T) {
	type fields struct {
		bookingService *mocks.MockBookingService
	}

	userID := uuid.New()
	bookingID := uuid.New()

	tests := []struct {
		name    string
		prepare func(f fields)
		ctx     context.Context
		request api.PostBookingsBookingIdCancelRequestObject
		want    api.PostBookingsBookingIdCancelResponseObject
	}{
		{
			name: "Success_Cancel",
			ctx:  setUserInContext(context.Background(), userID.String(), models.RoleUser),
			request: api.PostBookingsBookingIdCancelRequestObject{
				BookingId: bookingID,
			},
			prepare: func(f fields) {
				f.bookingService.EXPECT().
					Cancel(gomock.Any(), bookingID, userID).
					Return(&models.Booking{
						ID:     bookingID,
						UserID: userID,
					}, nil)
			},
			want: api.PostBookingsBookingIdCancel200JSONResponse{
				Booking: &api.Booking{
					Id:     bookingID,
					UserId: userID,
				},
			},
		},
		{
			name: "Failure_Unauthorized",
			ctx:  context.Background(),
			request: api.PostBookingsBookingIdCancelRequestObject{
				BookingId: bookingID,
			},
			prepare: func(f fields) {},
			want: api.PostBookingsBookingIdCancel401JSONResponse{
				Error: handler.MakeError(api.UNAUTHORIZED, handler.MsgUnauthorized),
			},
		},
		{
			name: "Failure_Invalid_UserID_In_Context",
			ctx:  setUserInContext(context.Background(), "bad-uuid", models.RoleUser),
			request: api.PostBookingsBookingIdCancelRequestObject{
				BookingId: bookingID,
			},
			prepare: func(f fields) {},
			want: api.PostBookingsBookingIdCancel403JSONResponse{
				Error: handler.MakeError(api.INVALIDREQUEST, handler.MsgInvalidUserID),
			},
		},
		{
			name: "Failure_NotFound",
			ctx:  setUserInContext(context.Background(), userID.String(), models.RoleUser),
			request: api.PostBookingsBookingIdCancelRequestObject{
				BookingId: bookingID,
			},
			prepare: func(f fields) {
				f.bookingService.EXPECT().
					Cancel(gomock.Any(), bookingID, userID).
					Return(nil, service.ErrNotFound)
			},
			want: api.PostBookingsBookingIdCancel404JSONResponse{
				Error: handler.MakeError(api.BOOKINGNOTFOUND, handler.MsgBookingNotFound),
			},
		},
		{
			name: "Failure_Forbidden_NotOwner",
			ctx:  setUserInContext(context.Background(), userID.String(), models.RoleUser),
			request: api.PostBookingsBookingIdCancelRequestObject{
				BookingId: bookingID,
			},
			prepare: func(f fields) {
				f.bookingService.EXPECT().
					Cancel(gomock.Any(), bookingID, userID).
					Return(nil, service.ErrForbidden)
			},
			want: api.PostBookingsBookingIdCancel403JSONResponse{
				Error: handler.MakeError(api.FORBIDDEN, handler.MsgCancelForbidden),
			},
		},
		{
			name: "Failure_Internal_Error",
			ctx:  setUserInContext(context.Background(), userID.String(), models.RoleUser),
			request: api.PostBookingsBookingIdCancelRequestObject{
				BookingId: bookingID,
			},
			prepare: func(f fields) {
				f.bookingService.EXPECT().
					Cancel(gomock.Any(), bookingID, userID).
					Return(nil, errors.New("sudden database failure"))
			},
			want: api.PostBookingsBookingIdCancel500JSONResponse{
				Error: handler.MakeInternalError(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := fields{
				bookingService: mocks.NewMockBookingService(ctrl),
			}
			tt.prepare(f)

			h := handler.NewHandler(f.bookingService, nil, nil, nil, 10, 50, 1, 1, 1)

			ctx := logger.ToContext(tt.ctx, zaptest.NewLogger(t))
			got, err := h.PostBookingsBookingIdCancel(ctx, tt.request)

			assert.NoError(t, err)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandler_PostDummyLogin(t *testing.T) {
	type fields struct {
		userService *mocks.MockUserService
		authService *mocks.MockTokenGenerator
	}

	dummyUser := &models.User{
		ID:   uuid.New(),
		Role: models.RoleUser,
	}
	testToken := "header.payload.signature"

	tests := []struct {
		name    string
		prepare func(f fields)
		request api.PostDummyLoginRequestObject
		want    api.PostDummyLoginResponseObject
	}{
		{
			name: "Success_Login",
			request: api.PostDummyLoginRequestObject{
				Body: &api.PostDummyLoginJSONRequestBody{
					Role: api.PostDummyLoginJSONBodyRole(api.UserRoleUser),
				},
			},
			prepare: func(f fields) {
				f.userService.EXPECT().
					GetOrCreateDummy(gomock.Any(), models.UserRole(api.UserRoleUser)).
					Return(dummyUser, nil)

				f.authService.EXPECT().
					GenerateToken(gomock.Any(), dummyUser).
					Return(testToken, nil)
			},
			want: api.PostDummyLogin200JSONResponse{
				Token: testToken,
			},
		},
		{
			name: "Failure_InvalidRole",
			request: api.PostDummyLoginRequestObject{
				Body: &api.PostDummyLoginJSONRequestBody{
					Role: "invalid_role",
				},
			},
			prepare: func(f fields) {
			},
			want: api.PostDummyLogin400JSONResponse{
				Error: handler.MakeError(api.INVALIDREQUEST, handler.MsgInvalidRole),
			},
		},
		{
			name: "Failure_UserService_Error",
			request: api.PostDummyLoginRequestObject{
				Body: &api.PostDummyLoginJSONRequestBody{
					Role: api.PostDummyLoginJSONBodyRole(api.UserRoleAdmin),
				},
			},
			prepare: func(f fields) {
				f.userService.EXPECT().
					GetOrCreateDummy(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("db error"))
			},
			want: api.PostDummyLogin500JSONResponse{
				Error: handler.MakeInternalError(),
			},
		},
		{
			name: "Failure_TokenService_Error",
			request: api.PostDummyLoginRequestObject{
				Body: &api.PostDummyLoginJSONRequestBody{
					Role: api.PostDummyLoginJSONBodyRole(api.UserRoleUser),
				},
			},
			prepare: func(f fields) {
				f.userService.EXPECT().
					GetOrCreateDummy(gomock.Any(), gomock.Any()).
					Return(dummyUser, nil)

				f.authService.EXPECT().
					GenerateToken(gomock.Any(), dummyUser).
					Return("", errors.New("jwt sign error"))
			},
			want: api.PostDummyLogin500JSONResponse{
				Error: handler.MakeInternalError(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := fields{
				userService: mocks.NewMockUserService(ctrl),
				authService: mocks.NewMockTokenGenerator(ctrl),
			}
			tt.prepare(f)

			h := handler.NewHandler(nil, nil, f.userService, f.authService, 10, 50, 1, 1, 1)

			ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))
			got, err := h.PostDummyLogin(ctx, tt.request)

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandler_PostLogin(t *testing.T) {
	type fields struct {
		userService *mocks.MockUserService
		authService *mocks.MockTokenGenerator
	}

	testEmail := "test@example.com"
	testPass := "secure_password"
	testToken := "jwt.token.here"

	validUser := &models.User{
		ID:    uuid.New(),
		Email: testEmail,
		Role:  models.RoleUser,
	}

	tests := []struct {
		name    string
		prepare func(f fields)
		request api.PostLoginRequestObject
		want    api.PostLoginResponseObject
	}{
		{
			name: "Success_Login",
			request: api.PostLoginRequestObject{
				Body: &api.PostLoginJSONRequestBody{
					Email:    types.Email(testEmail),
					Password: testPass,
				},
			},
			prepare: func(f fields) {
				f.userService.EXPECT().
					Login(gomock.Any(), models.UserLogin{
						Email:    testEmail,
						Password: testPass,
					}).
					Return(validUser, nil)

				f.authService.EXPECT().
					GenerateToken(gomock.Any(), validUser).
					Return(testToken, nil)
			},
			want: api.PostLogin200JSONResponse{
				Token: testToken,
			},
		},
		{
			name: "Invalid email",
			request: api.PostLoginRequestObject{
				Body: &api.PostLoginJSONRequestBody{
					Email:    types.Email("invalid email"),
					Password: "wrong_password",
				},
			},
			prepare: func(f fields) {
			},
			want: api.PostLogin401JSONResponse{
				Error: handler.MakeError(api.UNAUTHORIZED, handler.MsgInvalidCredentials),
			},
		},
		{
			name: "Invalid password",
			request: api.PostLoginRequestObject{
				Body: &api.PostLoginJSONRequestBody{
					Email:    types.Email(testEmail),
					Password: "pas",
				},
			},
			prepare: func(f fields) {
			},
			want: api.PostLogin401JSONResponse{
				Error: handler.MakeError(api.UNAUTHORIZED, handler.MsgInvalidCredentials),
			},
		},
		{
			name: "Failure_InvalidCredentials",
			request: api.PostLoginRequestObject{
				Body: &api.PostLoginJSONRequestBody{
					Email:    types.Email(testEmail),
					Password: "wrong_password",
				},
			},
			prepare: func(f fields) {
				f.userService.EXPECT().
					Login(gomock.Any(), gomock.Any()).
					Return(nil, service.ErrInvalidEmailOrPassword)
			},
			want: api.PostLogin401JSONResponse{
				Error: handler.MakeError(api.UNAUTHORIZED, handler.MsgInvalidCredentials),
			},
		},
		{
			name: "Failure_LoginService_InternalError",
			request: api.PostLoginRequestObject{
				Body: &api.PostLoginJSONRequestBody{
					Email:    types.Email(testEmail),
					Password: testPass,
				},
			},
			prepare: func(f fields) {
				f.userService.EXPECT().
					Login(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("database connection lost"))
			},
			want: api.PostLogin500JSONResponse{
				Error: handler.MakeInternalError(),
			},
		},
		{
			name: "Failure_TokenGeneration_Error",
			request: api.PostLoginRequestObject{
				Body: &api.PostLoginJSONRequestBody{
					Email:    types.Email(testEmail),
					Password: testPass,
				},
			},
			prepare: func(f fields) {
				f.userService.EXPECT().
					Login(gomock.Any(), gomock.Any()).
					Return(validUser, nil)

				f.authService.EXPECT().
					GenerateToken(gomock.Any(), validUser).
					Return("", errors.New("signing error"))
			},
			want: api.PostLogin500JSONResponse{
				Error: handler.MakeInternalError(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := fields{
				userService: mocks.NewMockUserService(ctrl),
				authService: mocks.NewMockTokenGenerator(ctrl),
			}
			tt.prepare(f)

			h := handler.NewHandler(nil, nil, f.userService, f.authService, 10, 50, 1, 1, 1)

			ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))
			got, err := h.PostLogin(ctx, tt.request)

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandler_PostRegister(t *testing.T) {
	type fields struct {
		userService *mocks.MockUserService
	}

	testEmail := "new@example.com"
	testPass := "secure_password"
	userID := uuid.New()

	validUser := &models.User{
		ID:    userID,
		Email: testEmail,
		Role:  models.RoleUser,
	}

	tests := []struct {
		name    string
		prepare func(f fields)
		request api.PostRegisterRequestObject
		want    api.PostRegisterResponseObject
	}{
		{
			name: "Success_Created",
			request: api.PostRegisterRequestObject{
				Body: &api.PostRegisterJSONRequestBody{
					Email:    types.Email(testEmail),
					Password: testPass,
					Role:     api.PostRegisterJSONBodyRole(api.UserRoleUser),
				},
			},
			prepare: func(f fields) {
				f.userService.EXPECT().
					Register(gomock.Any(), models.UserRegister{
						Email:    testEmail,
						Password: testPass,
						Role:     models.RoleUser,
					}).
					Return(validUser, nil)
			},
			want: api.PostRegister201JSONResponse{
				User: &api.User{
					Id:        userID,
					Email:     types.Email(testEmail),
					Role:      api.UserRoleUser,
					CreatedAt: &time.Time{},
				},
			},
		},
		{
			name: "Failure_InvalidRole",
			request: api.PostRegisterRequestObject{
				Body: &api.PostRegisterJSONRequestBody{
					Email:    types.Email(testEmail),
					Password: testPass,
					Role:     "hacker_role",
				},
			},
			prepare: func(f fields) {
			},
			want: api.PostRegister400JSONResponse{
				Error: handler.MakeError(api.INVALIDREQUEST, handler.MsgInvalidRole),
			},
		},
		{
			name: "Failure_ValidationError_Or_Conflict",
			request: api.PostRegisterRequestObject{
				Body: &api.PostRegisterJSONRequestBody{
					Email:    types.Email(testEmail),
					Password: "some_password",
					Role:     api.PostRegisterJSONBodyRole(api.UserRoleUser),
				},
			},
			prepare: func(f fields) {
				f.userService.EXPECT().
					Register(gomock.Any(), gomock.Any()).
					Return(nil, service.ErrInvalidEmailOrPassword)
			},
			want: api.PostRegister400JSONResponse{
				Error: handler.MakeError(api.INVALIDREQUEST, handler.MsgInvalidRegistration),
			},
		},
		{
			name: "Failure_InternalError",
			request: api.PostRegisterRequestObject{
				Body: &api.PostRegisterJSONRequestBody{
					Email:    types.Email(testEmail),
					Password: testPass,
					Role:     api.PostRegisterJSONBodyRole(api.UserRoleUser),
				},
			},
			prepare: func(f fields) {
				f.userService.EXPECT().
					Register(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("db error"))
			},
			want: api.PostRegister500JSONResponse{
				Error: handler.MakeInternalError(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := fields{
				userService: mocks.NewMockUserService(ctrl),
			}
			tt.prepare(f)

			h := handler.NewHandler(nil, nil, f.userService, nil, 10, 50, 1, 1, 1)

			ctx := logger.ToContext(context.Background(), zaptest.NewLogger(t))
			got, err := h.PostRegister(ctx, tt.request)

			assert.NoError(t, err)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandler_PostLanesCreate(t *testing.T) {
	type fields struct {
		LaneService *mocks.MockLaneService
	}

	adminID := uuid.New().String()
	userID := uuid.New().String()
	laneID := uuid.New()

	description := "Some lane"

	laneReq := models.LaneCreate{
		Name:        "Some bowling lane A",
		Description: &description,
		Type:        models.LaneTypeStandard,
	}

	validLane := &models.Lane{
		ID:          laneID,
		Name:        laneReq.Name,
		Description: laneReq.Description,
		Type:        laneReq.Type,
	}

	tests := []struct {
		name    string
		prepare func(f fields)
		ctx     context.Context
		request api.PostLanesCreateRequestObject
		want    api.PostLanesCreateResponseObject
	}{
		{
			name: "Success_Created_By_Admin",
			ctx:  setUserInContext(context.Background(), adminID, models.RoleAdmin),
			request: api.PostLanesCreateRequestObject{
				Body: &api.PostLanesCreateJSONRequestBody{
					Name:        laneReq.Name,
					Description: laneReq.Description,
					Type:        api.PostLanesCreateJSONBodyTypeStandard,
				},
			},
			prepare: func(f fields) {
				f.LaneService.EXPECT().
					CreateLane(gomock.Any(), models.LaneCreate{
						Name:        laneReq.Name,
						Description: laneReq.Description,
						Type:        laneReq.Type,
					}).
					Return(validLane, nil)
			},
			want: api.PostLanesCreate201JSONResponse{
				Lane: &api.Lane{
					Id:          laneID,
					Name:        laneReq.Name,
					Description: laneReq.Description,
					Type:        (*api.LaneType)(&laneReq.Type),
					CreatedAt:   &time.Time{},
				},
			},
		},
		{
			name: "Failure_Forbidden_RegularUser",
			ctx:  setUserInContext(context.Background(), userID, models.RoleUser),
			request: api.PostLanesCreateRequestObject{
				Body: &api.PostLanesCreateJSONRequestBody{
					Name:        laneReq.Name,
					Description: laneReq.Description,
					Type:        api.PostLanesCreateJSONBodyTypeStandard,
				},
			},
			prepare: func(f fields) {

			},
			want: api.PostLanesCreate403JSONResponse{
				Error: handler.MakeError(api.FORBIDDEN, handler.MsgForbiddenOnlyAdmins),
			},
		},
		{
			name: "Failure_Unauthorized",
			ctx:  context.Background(),
			request: api.PostLanesCreateRequestObject{
				Body: &api.PostLanesCreateJSONRequestBody{
					Name:        laneReq.Name,
					Description: laneReq.Description,
					Type:        api.PostLanesCreateJSONBodyTypeStandard,
				},
			},
			prepare: func(f fields) {},
			want: api.PostLanesCreate401JSONResponse{
				Error: handler.MakeError(api.UNAUTHORIZED, handler.MsgUnauthorized),
			},
		},
		{
			name: "Failure_InternalError",
			ctx:  setUserInContext(context.Background(), adminID, models.RoleAdmin),
			request: api.PostLanesCreateRequestObject{
				Body: &api.PostLanesCreateJSONRequestBody{
					Name:        laneReq.Name,
					Description: laneReq.Description,
					Type:        api.PostLanesCreateJSONBodyTypeStandard,
				},
			},
			prepare: func(f fields) {
				f.LaneService.EXPECT().
					CreateLane(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("persistence error"))
			},
			want: api.PostLanesCreate500JSONResponse{
				Error: handler.MakeInternalError(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := fields{
				LaneService: mocks.NewMockLaneService(ctrl),
			}
			tt.prepare(f)

			h := handler.NewHandler(nil, f.LaneService, nil, nil, 10, 50, 1, 1, 1)

			ctx := logger.ToContext(tt.ctx, zaptest.NewLogger(t))
			got, err := h.PostLanesCreate(ctx, tt.request)

			assert.NoError(t, err)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandler_GetLanesList(t *testing.T) {
	type fields struct {
		LaneService *mocks.MockLaneService
	}

	userID := uuid.New().String()
	LaneID := uuid.New()

	tests := []struct {
		name    string
		prepare func(f fields)
		ctx     context.Context
		request api.GetLanesListRequestObject
		want    api.GetLanesListResponseObject
	}{
		{
			name:    "Success_GetLanes",
			ctx:     setUserInContext(context.Background(), userID, models.RoleUser),
			request: api.GetLanesListRequestObject{},
			prepare: func(f fields) {
				f.LaneService.EXPECT().
					GetAllLanes(gomock.Any()).
					Return([]models.Lane{
						{
							ID:   LaneID,
							Name: "Main Hall",
							Type: models.LaneTypeStandard,
						},
					}, nil)
			},
			want: api.GetLanesList200JSONResponse{
				Lanes: &[]api.Lane{
					{
						Id:        LaneID,
						Name:      "Main Hall",
						Type:      ptr(api.LaneTypeStandard),
						CreatedAt: &time.Time{},
					},
				},
			},
		},
		{
			name:    "Success_EmptyList",
			ctx:     setUserInContext(context.Background(), userID, models.RoleUser),
			request: api.GetLanesListRequestObject{},
			prepare: func(f fields) {
				f.LaneService.EXPECT().
					GetAllLanes(gomock.Any()).
					Return([]models.Lane{}, nil)
			},
			want: api.GetLanesList200JSONResponse{
				Lanes: &[]api.Lane{},
			},
		},
		{
			name:    "Failure_Unauthorized",
			ctx:     context.Background(),
			request: api.GetLanesListRequestObject{},
			prepare: func(f fields) {},
			want: api.GetLanesList401JSONResponse{
				Error: handler.MakeError(api.UNAUTHORIZED, handler.MsgUnauthorized),
			},
		},
		{
			name:    "Failure_ServiceError",
			ctx:     setUserInContext(context.Background(), userID, models.RoleUser),
			request: api.GetLanesListRequestObject{},
			prepare: func(f fields) {
				f.LaneService.EXPECT().
					GetAllLanes(gomock.Any()).
					Return(nil, errors.New("db error"))
			},
			want: api.GetLanesList500JSONResponse{
				Error: handler.MakeInternalError(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := fields{
				LaneService: mocks.NewMockLaneService(ctrl),
			}
			tt.prepare(f)

			h := handler.NewHandler(nil, f.LaneService, nil, nil, 10, 50, 1, 1, 1)

			ctx := logger.ToContext(tt.ctx, zaptest.NewLogger(t))
			got, err := h.GetLanesList(ctx, tt.request)

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandler_PostLanesLaneIdScheduleCreate(t *testing.T) {
	type fields struct {
		LaneService *mocks.MockLaneService
	}

	adminID := uuid.New().String()
	userID := uuid.New().String()
	LaneID := uuid.New()

	validReqBody := api.PostLanesLaneIdScheduleCreateJSONRequestBody{
		DaysOfWeek: []int{1, 3, 5},
		StartTime:  "09:00",
		EndTime:    "18:00",
	}

	tests := []struct {
		name    string
		prepare func(f fields)
		ctx     context.Context
		request api.PostLanesLaneIdScheduleCreateRequestObject
		want    api.PostLanesLaneIdScheduleCreateResponseObject
	}{
		{
			name: "Success_Created",
			ctx:  setUserInContext(context.Background(), adminID, models.RoleAdmin),
			request: api.PostLanesLaneIdScheduleCreateRequestObject{
				LaneId: LaneID,
				Body:   &validReqBody,
			},
			prepare: func(f fields) {
				f.LaneService.EXPECT().
					CreateSchedule(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			want: api.PostLanesLaneIdScheduleCreate201JSONResponse{
				Schedule: &api.Schedule{
					Id:         &uuid.Nil,
					LaneId:     LaneID,
					DaysOfWeek: validReqBody.DaysOfWeek,
					StartTime:  validReqBody.StartTime,
					EndTime:    validReqBody.EndTime,
				},
			},
		},
		{
			name: "Failure_Forbidden_NotAdmin",
			ctx:  setUserInContext(context.Background(), userID, models.RoleUser),
			request: api.PostLanesLaneIdScheduleCreateRequestObject{
				LaneId: LaneID,
				Body:   &validReqBody,
			},
			want: api.PostLanesLaneIdScheduleCreate403JSONResponse{
				Error: handler.MakeError(api.FORBIDDEN, handler.MsgForbiddenOnlyAdmins),
			},
		},
		{
			name: "Failure_EmptyDaysOfWeek",
			ctx:  setUserInContext(context.Background(), adminID, models.RoleAdmin),
			request: api.PostLanesLaneIdScheduleCreateRequestObject{
				LaneId: LaneID,
				Body: &api.PostLanesLaneIdScheduleCreateJSONRequestBody{
					DaysOfWeek: []int{},
				},
			},
			want: api.PostLanesLaneIdScheduleCreate400JSONResponse{
				Error: handler.MakeError(api.INVALIDREQUEST, handler.MsgInvalidDaysOfWeek),
			},
		},
		{
			name: "Failure_InvalidDayRange",
			ctx:  setUserInContext(context.Background(), adminID, models.RoleAdmin),
			request: api.PostLanesLaneIdScheduleCreateRequestObject{
				LaneId: LaneID,
				Body: &api.PostLanesLaneIdScheduleCreateJSONRequestBody{
					DaysOfWeek: []int{0, 8},
				},
			},
			want: api.PostLanesLaneIdScheduleCreate400JSONResponse{
				Error: handler.MakeError(api.INVALIDREQUEST, handler.MsgInvalidDaysOfWeek),
			},
		},
		{
			name: "Failure_LaneNotFound",
			ctx:  setUserInContext(context.Background(), adminID, models.RoleAdmin),
			request: api.PostLanesLaneIdScheduleCreateRequestObject{
				LaneId: LaneID,
				Body:   &validReqBody,
			},
			prepare: func(f fields) {
				f.LaneService.EXPECT().
					CreateSchedule(gomock.Any(), gomock.Any()).
					Return(service.ErrLaneNotFound)
			},
			want: api.PostLanesLaneIdScheduleCreate404JSONResponse{
				Error: handler.MakeError(api.LANENOTFOUND, handler.MsgLaneNotFound),
			},
		},
		{
			name: "Failure_AlreadyExists_409",
			ctx:  setUserInContext(context.Background(), adminID, models.RoleAdmin),
			request: api.PostLanesLaneIdScheduleCreateRequestObject{
				LaneId: LaneID,
				Body:   &validReqBody,
			},
			prepare: func(f fields) {
				f.LaneService.EXPECT().
					CreateSchedule(gomock.Any(), gomock.Any()).
					Return(service.ErrAlreadyExists)
			},
			want: api.PostLanesLaneIdScheduleCreate409JSONResponse{
				Error: handler.MakeError(api.SCHEDULEEXISTS, handler.MsgScheduleExists),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := fields{
				LaneService: mocks.NewMockLaneService(ctrl),
			}
			if tt.prepare != nil {
				tt.prepare(f)
			}

			h := handler.NewHandler(
				nil, f.LaneService, nil, nil,
				10, 50, 1, 1, 1,
			)

			ctx := logger.ToContext(tt.ctx, zaptest.NewLogger(t))
			got, err := h.PostLanesLaneIdScheduleCreate(ctx, tt.request)

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandler_GetLanesLaneIdSlotsList(t *testing.T) {
	type fields struct {
		LaneService *mocks.MockLaneService
	}

	userID := uuid.New().String()
	LaneID := uuid.New()
	slotID := uuid.New()

	now := time.Now().Truncate(24 * time.Hour)
	testDate := types.Date{Time: now}

	tests := []struct {
		name    string
		prepare func(f fields)
		ctx     context.Context
		request api.GetLanesLaneIdSlotsListRequestObject
		want    api.GetLanesLaneIdSlotsListResponseObject
	}{
		{
			name: "Success_GetSlots",
			ctx:  setUserInContext(context.Background(), userID, models.RoleUser),
			request: api.GetLanesLaneIdSlotsListRequestObject{
				LaneId: LaneID,
				Params: api.GetLanesLaneIdSlotsListParams{
					Date: testDate,
				},
			},
			prepare: func(f fields) {
				f.LaneService.EXPECT().
					GetAvailableSlots(gomock.Any(), LaneID, gomock.Any()).
					Return([]models.Slot{
						{
							ID:        slotID,
							LaneID:    LaneID,
							StartTime: now.Add(9 * time.Hour),
							EndTime:   now.Add(10 * time.Hour),
						},
					}, nil)
			},
			want: api.GetLanesLaneIdSlotsList200JSONResponse{
				Slots: &[]api.Slot{
					{
						Id:     slotID,
						LaneId: LaneID,
						Start:  now.Add(9 * time.Hour),
						End:    now.Add(10 * time.Hour),
					},
				},
			},
		},
		{
			name: "Failure_LaneNotFound",
			ctx:  setUserInContext(context.Background(), userID, models.RoleUser),
			request: api.GetLanesLaneIdSlotsListRequestObject{
				LaneId: LaneID,
				Params: api.GetLanesLaneIdSlotsListParams{
					Date: testDate,
				},
			},
			prepare: func(f fields) {
				f.LaneService.EXPECT().
					GetAvailableSlots(gomock.Any(), LaneID, gomock.Any()).
					Return(nil, service.ErrLaneNotFound)
			},
			want: api.GetLanesLaneIdSlotsList404JSONResponse{
				Error: handler.MakeError(api.LANENOTFOUND, handler.MsgLaneNotFound),
			},
		},
		{
			name: "Failure_Unauthorized",
			ctx:  context.Background(),
			request: api.GetLanesLaneIdSlotsListRequestObject{
				LaneId: LaneID,
				Params: api.GetLanesLaneIdSlotsListParams{
					Date: testDate,
				},
			},
			prepare: func(f fields) {},
			want: api.GetLanesLaneIdSlotsList401JSONResponse{
				Error: handler.MakeError(api.UNAUTHORIZED, handler.MsgUnauthorized),
			},
		},
		{
			name: "Failure_InternalError",
			ctx:  setUserInContext(context.Background(), userID, models.RoleUser),
			request: api.GetLanesLaneIdSlotsListRequestObject{
				LaneId: LaneID,
				Params: api.GetLanesLaneIdSlotsListParams{
					Date: testDate,
				},
			},
			prepare: func(f fields) {
				f.LaneService.EXPECT().
					GetAvailableSlots(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("db error"))
			},
			want: api.GetLanesLaneIdSlotsList500JSONResponse{
				Error: handler.MakeInternalError(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := fields{
				LaneService: mocks.NewMockLaneService(ctrl),
			}
			tt.prepare(f)

			h := handler.NewHandler(nil, f.LaneService, nil, nil, 10, 50, 1, 1, 1)

			ctx := logger.ToContext(tt.ctx, zaptest.NewLogger(t))
			got, err := h.GetLanesLaneIdSlotsList(ctx, tt.request)

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func setUserInContext(ctx context.Context, id string, role models.UserRole) context.Context {
	claims := &models.Claims{
		UserID: id,
		Role:   role,
	}
	return context.WithValue(ctx, handler.UserKey, claims)
}

func ptr[T any](v T) *T {
	return &v
}
