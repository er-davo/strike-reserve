CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE user_role AS ENUM ('admin', 'user');

CREATE TYPE booking_status AS ENUM ('active', 'cancelled');

CREATE TYPE lane_type AS ENUM ('standard', 'vip', 'pro', 'kids');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    role user_role NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

CREATE TABLE lanes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type lane_type NOT NULL DEFAULT 'standard',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

CREATE TABLE schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lane_id UUID NOT NULL REFERENCES lanes(id) ON DELETE CASCADE,
    days_of_week INT[] NOT NULL, 
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    
    CONSTRAINT check_times CHECK (start_time < end_time),
    CONSTRAINT unique_schedule UNIQUE (lane_id)
);

CREATE INDEX idx_schedules_lane_id ON schedules(lane_id);

CREATE TABLE slots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lane_id UUID NOT NULL REFERENCES lanes(id) ON DELETE CASCADE,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    
    CONSTRAINT unique_slot UNIQUE (lane_id, start_time),
    CONSTRAINT check_slot_duration CHECK (end_time > start_time)
);

CREATE INDEX idx_slots_lane_start ON slots(lane_id, start_time);

CREATE TABLE bookings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    slot_id UUID NOT NULL REFERENCES slots(id) ON DELETE CASCADE,
    status booking_status NOT NULL,
    conference_link TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

CREATE UNIQUE INDEX unique_active_booking_per_slot
ON bookings(slot_id)
WHERE status = 'active';

CREATE INDEX idx_bookings_user_active
ON bookings(user_id)
WHERE status = 'active';