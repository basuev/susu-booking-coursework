CREATE TABLE booking_offer_snapshot (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    booking_id      UUID   NOT NULL REFERENCES booking(id) ON DELETE CASCADE,
    offer_id        UUID   NOT NULL,
    hotel_id        UUID   NOT NULL,
    room_type       TEXT   NOT NULL,
    price_per_night BIGINT NOT NULL CHECK (price_per_night > 0),
    currency        TEXT   NOT NULL,

    CONSTRAINT uq_booking_offer UNIQUE (booking_id)
);
