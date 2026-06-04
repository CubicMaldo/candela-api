CREATE TABLE businesses (
    business_id   UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_name    VARCHAR(255) NOT NULL,
    contact_email VARCHAR(255) NOT NULL,
    plan          VARCHAR(20)  NOT NULL DEFAULT 'FREE'
                  CHECK (plan IN ('FREE','BASIC','PREMIUM')),
    is_verified   BOOLEAN     NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE places (
    place_id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    name              VARCHAR(255) NOT NULL,
    description       TEXT,
    category          VARCHAR(20)  NOT NULL
                      CHECK (category IN (
                          'RESTAURANT','CAFE','BAR','NIGHTCLUB',
                          'PARK','STORE','UNIVERSITY','EVENT','OTHER'
                      )),
    minor_suitability VARCHAR(10)  NOT NULL DEFAULT 'PENDING'
                      CHECK (minor_suitability IN ('YES','NO','PENDING')),
    location          GEOGRAPHY(POINT, 4326) NOT NULL,
    geofence_meters   INTEGER     NOT NULL DEFAULT 100
                      CHECK (geofence_meters BETWEEN 50 AND 200),
    business_id       UUID        REFERENCES businesses(business_id) ON DELETE SET NULL,
    is_verified       BOOLEAN     NOT NULL DEFAULT false,
    is_active         BOOLEAN     NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_places_location          ON places USING GIST(location);
CREATE INDEX idx_places_category          ON places(category);
CREATE INDEX idx_places_minor_suitability ON places(minor_suitability);
CREATE INDEX idx_places_active            ON places(is_active);
