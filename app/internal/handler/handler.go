// [go:generate] swag init -g ../../cmd/main.go --parseDependency --parseInternal -o ../docs
package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"booking-service/internal/api"
	"booking-service/internal/models"
	"booking-service/internal/service"
	"booking-service/pkg/logger"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var _ api.StrictServerInterface = &Handler{}

type Handler struct {
	bookingService BookingService
	laneService    LaneService
	userService    UserService
	authService    TokenGenerator

	defaultPageSize uint64
	maxPageSize     uint64
	minPageSize     uint64
	defaultPage     uint64
	minPage         uint64

	validator *validator.Validate
}

func NewHandler(
	bookingService BookingService,
	laneService LaneService,
	userService UserService,
	authService TokenGenerator,

	defaultPageSize uint64,
	maxPageSize uint64,
	minPageSize uint64,
	defaultPage uint64,
	minPage uint64,
) *Handler {
	return &Handler{
		bookingService: bookingService,
		laneService:    laneService,
		userService:    userService,
		authService:    authService,

		defaultPageSize: defaultPageSize,
		maxPageSize:     maxPageSize,
		minPageSize:     minPageSize,
		defaultPage:     defaultPage,
		minPage:         minPage,
		validator:       validator.New(),
	}
}

// L — короткий хелпер для получения обогащенного логера из контекста
func (h *Handler) L(ctx context.Context) *zap.Logger {
	return logger.FromContext(ctx)
}

// PostBookingsCreate (POST /bookings/create)
// @Summary Создать бронь на слот (только user)
// @Description Создаёт бронь для пользователя. Админ не может бронировать. Слот не должен быть в прошлом.
// @Tags Bookings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body api.PostBookingsCreateJSONRequestBody true "Параметры бронирования"
// @Success 201 {object} api.PostBookingsCreate201JSONResponse
// @Failure 400 {object} api.PostBookingsCreate400JSONResponse
// @Failure 401 {object} api.PostBookingsCreate401JSONResponse
// @Failure 403 {object} api.PostBookingsCreate403JSONResponse
// @Failure 404 {object} api.PostBookingsCreate404JSONResponse
// @Failure 409 {object} api.PostBookingsCreate409JSONResponse
// @Failure 500 {object} api.PostBookingsCreate500JSONResponse
// @Router /bookings/create [post]
func (h *Handler) PostBookingsCreate(ctx context.Context, request api.PostBookingsCreateRequestObject) (api.PostBookingsCreateResponseObject, error) {
	l := h.L(ctx)

	user, ok := getUser(ctx)
	if !ok {
		l.Warn("unauthorized booking attempt")
		return api.PostBookingsCreate401JSONResponse{
			Error: MakeError(api.UNAUTHORIZED, MsgUnauthorized),
		}, nil
	}

	if user.Role != models.RoleUser {
		l.Warn("forbidden: non-user role booking attempt", zap.String("role", string(user.Role)))
		return api.PostBookingsCreate403JSONResponse{
			Error: MakeError(api.FORBIDDEN, MsgForbiddenOnlyUsers),
		}, nil
	}

	userID, err := uuid.Parse(user.UserID)
	if err != nil {
		l.Error("invalid user id in context", zap.Error(err), zap.String("user_id", user.UserID))
		return api.PostBookingsCreate400JSONResponse{
			Error: MakeError(api.INVALIDREQUEST, MsgInvalidUserID),
		}, nil
	}

	l.Info(
		"creating booking",
		zap.String("slot_id", request.Body.SlotId.String()),
	)

	booking, err := h.bookingService.Create(ctx, userID, request.Body.SlotId)
	if err != nil {
		if errors.Is(err, service.ErrSlotNotExisting) {
			l.Warn(
				"booking failed: slot not found",
				zap.String("slot_id", request.Body.SlotId.String()),
			)
			return api.PostBookingsCreate404JSONResponse{
				Error: MakeError(api.NOTFOUND, MsgSlotNotFound),
			}, nil
		}
		if errors.Is(err, service.ErrSlotIsUsed) {
			l.Warn(
				"booking failed: slot already booked",
				zap.String("slot_id", request.Body.SlotId.String()),
			)
			return api.PostBookingsCreate409JSONResponse{
				Error: MakeError(api.SLOTALREADYBOOKED, MsgSlotAlreadyBooked),
			}, nil
		}

		l.Error("internal error creating booking", zap.Error(err))
		return api.PostBookingsCreate500JSONResponse{
			Error: MakeInternalError(),
		}, nil
	}

	l.Info("booking created", zap.String("booking_id", booking.ID.String()))
	return api.PostBookingsCreate201JSONResponse{
		Booking: toApiBooking(*booking),
	}, nil
}

// GetBookingsList (GET /bookings/list)
// @Summary Список всех броней с пагинацией (только admin)
// @Description Возвращает все брони в системе. Доступно только для роли admin.
// @Tags Bookings
// @Produce json
// @Security BearerAuth
// @Param page query int false "Номер страницы (начиная с 1)" default(1)
// @Param pageSize query int false "Количество записей (max 100)" default(20)
// @Success 200 {object} api.GetBookingsList200JSONResponse
// @Failure 400 {object} api.GetBookingsList400JSONResponse
// @Failure 403 {object} api.GetBookingsList403JSONResponse
// @Router /bookings/list [get]
func (h *Handler) GetBookingsList(ctx context.Context, request api.GetBookingsListRequestObject) (api.GetBookingsListResponseObject, error) {
	l := h.L(ctx)

	user, ok := getUser(ctx)
	if !ok {
		return api.GetBookingsList401JSONResponse{
			Error: MakeError(api.UNAUTHORIZED, MsgUnauthorized),
		}, nil
	}

	if user.Role != models.RoleAdmin {
		l.Warn("forbidden: non-admin list access", zap.String("user_id", user.ID))
		return api.GetBookingsList403JSONResponse{
			Error: MakeError(api.FORBIDDEN, MsgForbiddenOnlyAdmins),
		}, nil
	}

	page := h.defaultPage
	if request.Params.Page != nil {
		page = uint64(*request.Params.Page)
		if page < h.minPage {
			return api.GetBookingsList400JSONResponse{
				Error: MakeError(api.INVALIDREQUEST, MsgInvalidPage),
			}, nil
		}
	}

	pageSize := h.defaultPageSize
	if request.Params.PageSize != nil {
		pageSize = uint64(*request.Params.PageSize)
		if pageSize > h.maxPageSize || pageSize < h.minPageSize {
			return api.GetBookingsList400JSONResponse{
				Error: MakeError(api.INVALIDREQUEST, MsgInvalidPageSize),
			}, nil
		}
	}

	l.Debug("fetching all bookings", zap.Uint64("page", page), zap.Uint64("page_size", pageSize))

	bList, err := h.bookingService.GetList(ctx, pageSize, page-1)
	if err != nil {
		l.Error("failed to get bookings list", zap.Error(err))
		return api.GetBookingsList500JSONResponse{
			Error: MakeInternalError(),
		}, nil
	}

	apiBookings := make([]api.Booking, 0, len(bList.Bookings))
	for _, b := range bList.Bookings {
		apiBookings = append(apiBookings, *toApiBooking(b))
	}

	return api.GetBookingsList200JSONResponse{
		Bookings: &apiBookings,
		Pagination: &api.Pagination{
			Page:     int(page),
			PageSize: int(pageSize),
			Total:    int(bList.Total),
		},
	}, nil
}

// GetBookingsMy (GET /bookings/my)
// @Summary Список броней текущего пользователя (только user)
// @Description Возвращает будущие брони (start >= now) авторизованного пользователя.
// @Tags Bookings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} api.GetBookingsMy200JSONResponse
// @Failure 401 {object} api.GetBookingsMy401JSONResponse
// @Router /bookings/my [get]
func (h *Handler) GetBookingsMy(ctx context.Context, request api.GetBookingsMyRequestObject) (api.GetBookingsMyResponseObject, error) {
	l := h.L(ctx)

	user, ok := getUser(ctx)
	if !ok {
		return api.GetBookingsMy401JSONResponse{
			Error: MakeError(api.UNAUTHORIZED, MsgUnauthorized),
		}, nil
	}

	userID, err := uuid.Parse(user.UserID)
	if err != nil {
		l.Error("invalid user id in context", zap.Error(err), zap.String("user_id", user.UserID))
		// There is no 400JSONResponse generated
		// using 403 instead
		return api.GetBookingsMy403JSONResponse{
			Error: MakeError(api.INVALIDREQUEST, MsgInvalidUserID),
		}, nil
	}

	bookings, err := h.bookingService.GetByUserID(ctx, userID)
	if err != nil {
		l.Error("failed to get my bookings", zap.Error(err))
		return api.GetBookingsMy500JSONResponse{
			Error: MakeInternalError(),
		}, nil
	}

	apiBookings := make([]api.Booking, 0, len(bookings))
	for _, b := range bookings {
		apiBookings = append(apiBookings, *toApiBooking(b))
	}

	return api.GetBookingsMy200JSONResponse{
		Bookings: &apiBookings,
	}, nil
}

// PostBookingsBookingIdCancel (POST /bookings/{bookingId}/cancel)
// @Summary Отменить бронь (только своя, только user)
// @Description Идемпотентная операция отмены брони. Только для владельца брони.
// @Tags Bookings
// @Produce json
// @Security BearerAuth
// @Param bookingId path string true "Идентификатор брони" format(uuid)
// @Success 200 {object} api.PostBookingsBookingIdCancel200JSONResponse
// @Failure 403 {object} api.PostBookingsBookingIdCancel403JSONResponse
// @Failure 404 {object} api.PostBookingsBookingIdCancel404JSONResponse
// @Router /bookings/{bookingId}/cancel [post]
func (h *Handler) PostBookingsBookingIdCancel(ctx context.Context, request api.PostBookingsBookingIdCancelRequestObject) (api.PostBookingsBookingIdCancelResponseObject, error) {
	l := h.L(ctx).With(zap.String("booking_id", request.BookingId.String()))

	user, ok := getUser(ctx)
	if !ok {
		return api.PostBookingsBookingIdCancel401JSONResponse{
			Error: MakeError(api.UNAUTHORIZED, MsgUnauthorized),
		}, nil
	}

	userID, err := uuid.Parse(user.UserID)
	if err != nil {
		l.Error("invalid user id in context", zap.Error(err), zap.String("user_id", user.UserID))
		// There is no 400JSONResponse generated
		// using 403 instead
		return api.PostBookingsBookingIdCancel403JSONResponse{
			Error: MakeError(api.INVALIDREQUEST, MsgInvalidUserID),
		}, nil
	}

	l.Info("canceling booking")

	booking, err := h.bookingService.Cancel(ctx, request.BookingId, userID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			l.Warn("cancel failed: booking not found")
			return api.PostBookingsBookingIdCancel404JSONResponse{
				Error: MakeError(api.BOOKINGNOTFOUND, MsgBookingNotFound),
			}, nil
		}
		if errors.Is(err, service.ErrForbidden) {
			l.Warn("cancel forbidden: not owner", zap.String("user_id", user.UserID))
			return api.PostBookingsBookingIdCancel403JSONResponse{
				Error: MakeError(api.FORBIDDEN, MsgCancelForbidden),
			}, nil
		}

		l.Error("internal error canceling booking", zap.Error(err))
		return api.PostBookingsBookingIdCancel500JSONResponse{
			Error: MakeInternalError(),
		}, nil
	}

	l.Info("booking canceled")
	return api.PostBookingsBookingIdCancel200JSONResponse{
		Booking: toApiBooking(*booking),
	}, nil
}

// PostDummyLogin (POST /dummyLogin)
// @Summary Получить тестовый JWT по роли
// @Description Выдаёт фиксированный UUID для admin или user.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body api.PostDummyLoginJSONRequestBody true "Роль (admin/user)"
// @Success 200 {object} api.Token
// @Router /dummyLogin [post]
func (h *Handler) PostDummyLogin(ctx context.Context, request api.PostDummyLoginRequestObject) (api.PostDummyLoginResponseObject, error) {
	l := h.L(ctx).With(zap.String("role", string(request.Body.Role)))

	if !request.Body.Role.Valid() {
		l.Warn("invalid role in dummy login")
		return api.PostDummyLogin400JSONResponse{
			Error: MakeError(api.INVALIDREQUEST, MsgInvalidRole),
		}, nil
	}

	user, err := h.userService.GetOrCreateDummy(ctx, models.UserRole(request.Body.Role))
	if err != nil {
		l.Error("dummy login failed", zap.Error(err))
		return api.PostDummyLogin500JSONResponse{
			Error: MakeInternalError(),
		}, nil
	}

	token, err := h.authService.GenerateToken(ctx, user)
	if err != nil {
		l.Error("dummy token generation failed", zap.Error(err))
		return api.PostDummyLogin500JSONResponse{
			Error: MakeInternalError(),
		}, nil
	}

	l.Info("dummy login successful", zap.String("dummy_user_id", user.ID.String()))
	return api.PostDummyLogin200JSONResponse{
		Token: token,
	}, nil
}

// PostLogin (POST /login)
// @Summary Авторизация по email и паролю (Доп. задание)
// @Description Возвращает JWT. Доступно без авторизации.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body api.PostLoginJSONRequestBody true "Credentials"
// @Success 200 {object} api.Token
// @Failure 401 {object} api.ErrorResponse
// @Failure 500 {object} api.InternalErrorResponse
// @Router /login [post]
func (h *Handler) PostLogin(ctx context.Context, request api.PostLoginRequestObject) (api.PostLoginResponseObject, error) {
	l := h.L(ctx).With(zap.String("email", string(request.Body.Email)))

	ul := models.UserLogin{
		Email:    string(request.Body.Email),
		Password: request.Body.Password,
	}

	if err := h.validator.Struct(ul); err != nil {
		// There is no 400JSONresponse generated
		// using 401 instead
		return api.PostLogin401JSONResponse{
			Error: MakeError(api.UNAUTHORIZED, MsgInvalidCredentials),
		}, nil
	}

	user, err := h.userService.Login(ctx, ul)
	if err != nil {
		if errors.Is(err, service.ErrInvalidEmailOrPassword) {
			l.Warn("login failed: invalid credentials")
			return api.PostLogin401JSONResponse{
				Error: MakeError(api.UNAUTHORIZED, MsgInvalidCredentials),
			}, nil
		}

		l.Error("login internal error", zap.Error(err))
		return api.PostLogin500JSONResponse{
			Error: MakeInternalError(),
		}, nil
	}

	token, err := h.authService.GenerateToken(ctx, user)
	if err != nil {
		l.Error("login token generation failed", zap.Error(err))
		return api.PostLogin500JSONResponse{
			Error: MakeInternalError(),
		}, nil
	}

	l.Info("user login success", zap.String("user_id", user.ID.String()))
	return api.PostLogin200JSONResponse{
		Token: token,
	}, nil
}

// PostRegister (POST /register)
// @Summary Регистрация пользователя (Доп. задание)
// @Description Создаёт нового пользователя. Доступно без авторизации.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body api.PostRegisterJSONRequestBody true "Данные регистрации"
// @Success 201 {object} api.PostRegister201JSONResponse
// @Failure 400 {object} api.ErrorResponse
// @Failure 500 {object} api.InternalErrorResponse
// @Router /register [post]
func (h *Handler) PostRegister(ctx context.Context, request api.PostRegisterRequestObject) (api.PostRegisterResponseObject, error) {
	l := h.L(ctx).With(zap.String("email", string(request.Body.Email)))

	if !request.Body.Role.Valid() {
		l.Warn("invalid role in registration")
		return api.PostRegister400JSONResponse{
			Error: MakeError(api.INVALIDREQUEST, MsgInvalidRole),
		}, nil
	}

	ur := models.UserRegister{
		Email:    string(request.Body.Email),
		Password: request.Body.Password,
		Role:     models.UserRole(request.Body.Role),
	}

	if err := h.validator.Struct(ur); err != nil {
		return api.PostRegister400JSONResponse{
			Error: MakeError(api.INVALIDREQUEST, MsgInvalidRegistration),
		}, nil
	}

	user, err := h.userService.Register(ctx, ur)
	if err != nil {
		if errors.Is(err, service.ErrInvalidEmailOrPassword) {
			l.Warn("registration failed: invalid email/pass format or constraint")
			return api.PostRegister400JSONResponse{
				Error: MakeError(api.INVALIDREQUEST, MsgInvalidRegistration),
			}, nil
		}

		l.Error("registration internal error", zap.Error(err))
		return api.PostRegister500JSONResponse{
			Error: MakeInternalError(),
		}, nil
	}

	l.Info("user registered", zap.String("user_id", user.ID.String()))
	return api.PostRegister201JSONResponse{
		User: toApiUser(*user),
	}, nil
}

// PostRoomsCreate (POST /rooms/create)
// @Summary Создать переговорку (только admin)
// @Tags Rooms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body api.PostRoomsCreateJSONRequestBody true "Данные комнаты"
// @Success 201 {object} api.PostRoomsCreate201JSONResponse
// @Failure 403 {object} api.PostRoomsCreate403JSONResponse
// @Router /rooms/create [post]
func (h *Handler) PostLanesCreate(ctx context.Context, request api.PostLanesCreateRequestObject) (api.PostLanesCreateResponseObject, error) {
	l := h.L(ctx)

	if l == nil {
		fmt.Println("Logger is fucking nil")
	}

	user, ok := getUser(ctx)
	if !ok {
		return api.PostLanesCreate401JSONResponse{
			Error: MakeError(api.UNAUTHORIZED, MsgUnauthorized),
		}, nil
	}

	if user.Role != models.RoleAdmin {
		l.Warn("forbidden lane creation attempt", zap.String("user_id", user.ID))
		return api.PostLanesCreate403JSONResponse{
			Error: MakeError(api.FORBIDDEN, MsgForbiddenOnlyAdmins),
		}, nil
	}

	if !request.Body.Type.Valid() {
		return api.PostLanesCreate400JSONResponse{
			Error: MakeError(api.INVALIDREQUEST, MsgInvalidLaneType),
		}, nil
	}

	l.Info("creating room", zap.String("name", request.Body.Name))

	room, err := h.laneService.CreateLane(ctx, models.LaneCreate{
		Name:        request.Body.Name,
		Description: request.Body.Description,
		Type:        models.LaneType(request.Body.Type),
	})
	if err != nil {
		l.Error("failed to create room", zap.Error(err))
		return api.PostLanesCreate500JSONResponse{
			Error: MakeInternalError(),
		}, nil
	}

	l.Info("room created", zap.String("room_id", room.ID.String()))
	return api.PostLanesCreate201JSONResponse{
		Lane: toApiRoom(*room),
	}, nil
}

// GetRoomsList (GET /rooms/list)
// @Summary Список переговорок (admin и user)
// @Tags Rooms
// @Produce json
// @Security BearerAuth
// @Success 200 {object} api.GetRoomsList200JSONResponse
// @Router /rooms/list [get]
func (h *Handler) GetLanesList(ctx context.Context, request api.GetLanesListRequestObject) (api.GetLanesListResponseObject, error) {
	if _, ok := getUser(ctx); !ok {
		return api.GetLanesList401JSONResponse{
			Error: MakeError(api.UNAUTHORIZED, MsgUnauthorized),
		}, nil
	}

	lanes, err := h.laneService.GetAllLanes(ctx)
	if err != nil {
		h.L(ctx).Error("failed to get lanes list", zap.Error(err))

		return api.GetLanesList500JSONResponse{
			Error: MakeInternalError(),
		}, nil
	}

	apiRooms := make([]api.Lane, 0, len(lanes))
	for _, r := range lanes {
		apiRooms = append(apiRooms, *toApiRoom(r))
	}

	return api.GetLanesList200JSONResponse{
		Lanes: &apiRooms,
	}, nil
}

// PostRoomsRoomIdScheduleCreate (POST /rooms/{roomId}/schedule/create)
// @Summary Создать расписание переговорки (только admin)
// @Description Создается один раз. Поле daysOfWeek: 1 (Пн) - 7 (Вс).
// @Tags Schedules
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param roomId path string true "Идентификатор комнаты" format(uuid)
// @Param request body api.Schedule true "Объект расписания"
// @Success 201 {object} api.PostRoomsRoomIdScheduleCreate201JSONResponse
// @Failure 409 {object} api.PostRoomsRoomIdScheduleCreate409JSONResponse
// @Router /rooms/{roomId}/schedule/create [post]
func (h *Handler) PostLanesLaneIdScheduleCreate(ctx context.Context, request api.PostLanesLaneIdScheduleCreateRequestObject) (api.PostLanesLaneIdScheduleCreateResponseObject, error) {
	l := h.L(ctx).With(zap.String("room_id", request.LaneId.String()))

	user, ok := getUser(ctx)
	if !ok {
		return api.PostLanesLaneIdScheduleCreate401JSONResponse{
			Error: MakeError(api.UNAUTHORIZED, MsgUnauthorized),
		}, nil
	}

	if user.Role != models.RoleAdmin {
		l.Warn("forbidden schedule creation", zap.String("user_id", user.UserID))
		return api.PostLanesLaneIdScheduleCreate403JSONResponse{
			Error: MakeError(api.FORBIDDEN, MsgForbiddenOnlyAdmins),
		}, nil
	}

	l.Info("creating schedule")

	schedule := models.Schedule{
		LaneID:     request.LaneId,
		DaysOfWeek: request.Body.DaysOfWeek,
		StartTime:  models.FromHMToHMS(request.Body.StartTime),
		EndTime:    models.FromHMToHMS(request.Body.EndTime),
	}
	if err := h.validator.Struct(schedule); err != nil {
		return api.PostLanesLaneIdScheduleCreate400JSONResponse{
			Error: MakeError(api.INVALIDREQUEST, MsgInvalidDaysOfWeek),
		}, nil
	}

	if err := h.laneService.CreateSchedule(ctx, &schedule); err != nil {
		if errors.Is(err, service.ErrLaneNotFound) {
			l.Warn("schedule failed: room not found")
			return api.PostLanesLaneIdScheduleCreate404JSONResponse{
				Error: MakeError(api.LANENOTFOUND, MsgLaneNotFound),
			}, nil
		}
		if errors.Is(err, service.ErrAlreadyExists) {
			l.Warn("schedule failed: already exists")
			return api.PostLanesLaneIdScheduleCreate409JSONResponse{
				Error: MakeError(api.SCHEDULEEXISTS, MsgScheduleExists),
			}, nil
		}

		l.Error("failed to create schedule", zap.Error(err))
		return api.PostLanesLaneIdScheduleCreate500JSONResponse{
			Error: MakeInternalError(),
		}, nil
	}

	l.Info("schedule created")
	return api.PostLanesLaneIdScheduleCreate201JSONResponse{
		Schedule: toApiSchedule(schedule),
	}, nil
}

// GetRoomsRoomIdSlotsList (GET /rooms/{roomId}/slots/list)
// @Summary Список доступных слотов по дате
// @Description Возвращает свободные слоты на конкретный день. Параметр date обязателен.
// @Tags Slots
// @Produce json
// @Security BearerAuth
// @Param roomId path string true "Идентификатор комнаты" format(uuid)
// @Param date query string true "Дата (YYYY-MM-DD)" format(date)
// @Success 200 {object} api.GetRoomsRoomIdSlotsList200JSONResponse
// @Router /rooms/{roomId}/slots/list [get]
func (h *Handler) GetLanesLaneIdSlotsList(ctx context.Context, request api.GetLanesLaneIdSlotsListRequestObject) (api.GetLanesLaneIdSlotsListResponseObject, error) {
	l := h.L(ctx).With(zap.String("lane_id", request.LaneId.String()))

	if _, ok := getUser(ctx); !ok {
		return api.GetLanesLaneIdSlotsList401JSONResponse{
			Error: MakeError(api.UNAUTHORIZED, MsgUnauthorized),
		}, nil
	}

	year, month, day := request.Params.Date.Date()
	requestedDate := time.Date(year, month, day, 0, 0, 0, 0, request.Params.Date.Location())

	if requestedDate.IsZero() {
		l.Warn("invalid date format in slots request")
		return api.GetLanesLaneIdSlotsList400JSONResponse{
			Error: MakeError(api.INVALIDREQUEST, MsgInvalidDate),
		}, nil
	}

	l.Debug("fetching slots", zap.Time("date", requestedDate))

	slots, err := h.laneService.GetAvailableSlots(ctx, request.LaneId, requestedDate)
	if err != nil {
		if errors.Is(err, service.ErrLaneNotFound) {
			l.Warn("slots request failed: lane not found")
			return api.GetLanesLaneIdSlotsList404JSONResponse{
				Error: MakeError(api.LANENOTFOUND, MsgLaneNotFound),
			}, nil
		}

		l.Error("failed to get slots", zap.Error(err))
		return api.GetLanesLaneIdSlotsList500JSONResponse{
			Error: MakeInternalError(),
		}, nil
	}

	apiSlots := make([]api.Slot, 0, len(slots))
	for _, s := range slots {
		apiSlots = append(apiSlots, *toApiSlot(s))
	}

	return api.GetLanesLaneIdSlotsList200JSONResponse{
		Slots: &apiSlots,
	}, nil
}
