DROP INDEX IF EXISTS idx_bookings_user;
DROP INDEX IF EXISTS unique_active_booking_per_slot;
DROP INDEX IF EXISTS idx_slots_room_date;

DROP TABLE IF EXISTS bookings;
DROP TABLE IF EXISTS slots;
DROP TABLE IF EXISTS schedules;
DROP TABLE IF EXISTS lanes;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS booking_status;
DROP TYPE IF EXISTS user_role;
DROP TYPE IF EXISTS lane_type;

DROP EXTENSION IF EXISTS "pgcrypto";