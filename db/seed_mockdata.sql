-- ============================================================
-- Datos de prueba (mockup) para debugging, testing y demos
-- ============================================================
-- Uso:
--   docker compose exec -T postgres psql -U $POSTGRES_USER -d $POSTGRES_DB < db/seed_mockdata.sql
-- (o copiar/pegar en Adminer / psql)
--
-- Contraseña para TODOS los usuarios de prueba: "password123"
-- (hash bcrypt generado con bcrypt.DefaultCost)
-- ============================================================

BEGIN;

-- Limpieza de datos previos de prueba (orden por dependencias FK)
DELETE FROM reports;
DELETE FROM plans;
DELETE FROM follows;
DELETE FROM check_ins;
DELETE FROM friendships;
DELETE FROM point_transactions;
DELETE FROM places;
DELETE FROM businesses;
DELETE FROM users;

-- ── Usuarios ─────────────────────────────────────────────
-- password_hash corresponde a "password123"
INSERT INTO users (
    user_id, email, username, password_hash, birth_date, age,
    is_protected, account_status, privacy_mode, points_total, points_season,
    badge_level, streak_weeks_current, streak_weeks_max, ranking_opt_in
) VALUES
    ('11111111-1111-1111-1111-111111111111', 'ana.garcia@example.com',   'ana_garcia',   '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '1999-03-12', 27, false, 'ACTIVE', 'PUBLIC',  1240, 320, 'Explorador Veterano', 3, 5, true),
    ('22222222-2222-2222-2222-222222222222', 'carlos.ruiz@example.com',  'carlos_ruiz',  '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2001-07-23', 24, false, 'ACTIVE', 'FRIENDS', 860,  210, 'Explorador Activo',   1, 2, false),
    ('33333333-3333-3333-3333-333333333333', 'laura.fernandez@example.com', 'laura_fdez', '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2003-11-02', 22, false, 'ACTIVE', 'PUBLIC',  430,  120, 'Explorador Nuevo',    0, 1, true),
    ('44444444-4444-4444-4444-444444444444', 'mateo.lopez@example.com',  'mateo_lopez',  '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2008-01-15', 18, true,  'ACTIVE', 'FRIENDS', 95,   95,  'Explorador Nuevo',    0, 0, false),
    ('55555555-5555-5555-5555-555555555555', 'sofia.martinez@example.com', 'sofia_mtz',  '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '1997-09-30', 28, false, 'ACTIVE', 'PRIVATE', 2100, 540, 'Leyenda Local',       6, 8, true),
    ('66666666-6666-6666-6666-666666666666', 'pedro.sanchez@example.com', 'pedro_sanchez', '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2006-05-18', 20, false, 'PENDING', 'FRIENDS', 0,   0,   'Explorador Nuevo',    0, 0, false),

    -- Usuario admin (sin columna de rol en el esquema; se identifica por username/email
    -- y se usa como revisor en `reports.reviewed_by` para simular moderación)
    ('99999999-9999-9999-9999-999999999999', 'admin@candela.example.com', 'admin',        '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '1990-01-01', 36, false, 'ACTIVE', 'PRIVATE', 0,    0,   'Administrador',       0, 0, false);

-- ── Negocios ─────────────────────────────────────────────
INSERT INTO businesses (business_id, legal_name, contact_email, plan, is_verified) VALUES
    ('aaaaaaaa-0000-0000-0000-000000000001', 'Cafetería La Plaza S.L.',    'contacto@laplaza.example.com',   'PREMIUM', true),
    ('aaaaaaaa-0000-0000-0000-000000000002', 'Bar El Rincón S.L.',         'info@elrincon.example.com',      'BASIC',   true),
    ('aaaaaaaa-0000-0000-0000-000000000003', 'Club Nocturno Aurora S.L.',  'reservas@aurora.example.com',    'FREE',    false);

-- ── Lugares (coordenadas en Barranquilla) ────────────────
-- ST_MakePoint(longitud, latitud) — centro de referencia: 11.0173, -74.8111 (El Prado)
INSERT INTO places (place_id, name, description, category, minor_suitability, location, geofence_meters, business_id, is_verified, is_active) VALUES
    ('bbbbbbbb-0000-0000-0000-000000000001', 'Cafetería La Plaza',
        'Cafetería de especialidad en pleno centro, ideal para estudiar o quedar.',
        'CAFE', 'YES', ST_SetSRID(ST_MakePoint(-74.8080, 11.0110), 4326)::geography, 80,
        'aaaaaaaa-0000-0000-0000-000000000001', true, true),

    ('bbbbbbbb-0000-0000-0000-000000000002', 'Bar El Rincón',
        'Bar de tapas costeño con terraza y música en vivo.',
        'BAR', 'NO', ST_SetSRID(ST_MakePoint(-74.8200, 11.0220), 4326)::geography, 100,
        'aaaaaaaa-0000-0000-0000-000000000002', true, true),

    ('bbbbbbbb-0000-0000-0000-000000000003', 'Club Nocturno Aurora',
        'Sala de música electrónica y eventos nocturnos en la zona rosa.',
        'NIGHTCLUB', 'NO', ST_SetSRID(ST_MakePoint(-74.8050, 11.0250), 4326)::geography, 150,
        'aaaaaaaa-0000-0000-0000-000000000003', false, true),

    ('bbbbbbbb-0000-0000-0000-000000000004', 'Parque Cultural del Caribe',
        'Gran parque urbano con espacios culturales, perfecto para planes al aire libre.',
        'PARK', 'YES', ST_SetSRID(ST_MakePoint(-74.7990, 11.0040), 4326)::geography, 200,
        NULL, true, true),

    ('bbbbbbbb-0000-0000-0000-000000000005', 'Universidad del Norte',
        'Campus universitario con zonas comunes, cafeterías y biblioteca.',
        'UNIVERSITY', 'YES', ST_SetSRID(ST_MakePoint(-74.8510, 11.0350), 4326)::geography, 200,
        NULL, true, true),

    ('bbbbbbbb-0000-0000-0000-000000000006', 'Restaurante Sabor Local',
        'Cocina de mercado con sabores caribeños en ambiente acogedor.',
        'RESTAURANT', 'YES', ST_SetSRID(ST_MakePoint(-74.8120, 11.0200), 4326)::geography, 100,
        NULL, false, true);

-- ── Check-ins (todos dentro de la ventana activa de 4 horas) ─
INSERT INTO check_ins (
    checkin_id, user_id, place_id, gps_lat, gps_lon, gps_valid, timestamp_valid,
    validation_passed, points_awarded, is_first_of_day, checked_in_at
) VALUES
    ('cccccccc-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000001', 11.0110, -74.8080, true, true, true, 20, true,  NOW() - INTERVAL '30 minutes'),
    ('cccccccc-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000004', 11.0040, -74.7990, true, true, true, 15, true,  NOW() - INTERVAL '1 hour'),
    ('cccccccc-0000-0000-0000-000000000003', '22222222-2222-2222-2222-222222222222', 'bbbbbbbb-0000-0000-0000-000000000002', 11.0220, -74.8200, true, true, true, 20, true,  NOW() - INTERVAL '45 minutes'),
    ('cccccccc-0000-0000-0000-000000000004', '33333333-3333-3333-3333-333333333333', 'bbbbbbbb-0000-0000-0000-000000000006', 11.0200, -74.8120, true, true, true, 20, true,  NOW() - INTERVAL '2 hours'),
    ('cccccccc-0000-0000-0000-000000000005', '55555555-5555-5555-5555-555555555555', 'bbbbbbbb-0000-0000-0000-000000000003', 11.0250, -74.8050, true, true, true, 25, true,  NOW() - INTERVAL '1 hour 30 minutes'),
    ('cccccccc-0000-0000-0000-000000000006', '55555555-5555-5555-5555-555555555555', 'bbbbbbbb-0000-0000-0000-000000000001', 11.0110, -74.8080, true, true, true, 20, false, NOW() - INTERVAL '3 hours');

-- ── Follows (usuarios siguiendo lugares) ─────────────────
INSERT INTO follows (follower_id, place_id) VALUES
    ('11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000001'),
    ('11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000003'),
    ('22222222-2222-2222-2222-222222222222', 'bbbbbbbb-0000-0000-0000-000000000002'),
    ('33333333-3333-3333-3333-333333333333', 'bbbbbbbb-0000-0000-0000-000000000004'),
    ('55555555-5555-5555-5555-555555555555', 'bbbbbbbb-0000-0000-0000-000000000001'),
    ('55555555-5555-5555-5555-555555555555', 'bbbbbbbb-0000-0000-0000-000000000006');

-- ── Amistades (user_id_a < user_id_b por constraint) ─────
INSERT INTO friendships (user_id_a, user_id_b, status, requester_id) VALUES
    ('11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222', 'accepted', '11111111-1111-1111-1111-111111111111'),
    ('11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555555', 'accepted', '55555555-5555-5555-5555-555555555555'),
    ('22222222-2222-2222-2222-222222222222', '33333333-3333-3333-3333-333333333333', 'pending',  '22222222-2222-2222-2222-222222222222'),
    ('33333333-3333-3333-3333-333333333333', '55555555-5555-5555-5555-555555555555', 'pending',  '33333333-3333-3333-3333-333333333333'),
    ('22222222-2222-2222-2222-222222222222', '66666666-6666-6666-6666-666666666666', 'blocked',  '22222222-2222-2222-2222-222222222222');

-- ── Planes ───────────────────────────────────────────────
INSERT INTO plans (
    plan_id, creator_user_id, target_user_id, place_id, status, message,
    deadline_hours, deadline_at, completed_at, created_at
) VALUES
    ('dddddddd-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222',
        'bbbbbbbb-0000-0000-0000-000000000001', 'ACCEPTED', '¿Café mañana por la tarde?', 24,
        NOW() + INTERVAL '20 hours', NULL, NOW() - INTERVAL '4 hours'),

    ('dddddddd-0000-0000-0000-000000000002', '55555555-5555-5555-5555-555555555555', '11111111-1111-1111-1111-111111111111',
        'bbbbbbbb-0000-0000-0000-000000000004', 'COMPLETED', 'Paseo por el Parque Cultural este finde', 48,
        NOW() - INTERVAL '1 day', NOW() - INTERVAL '2 days', NOW() - INTERVAL '5 days'),

    ('dddddddd-0000-0000-0000-000000000003', '22222222-2222-2222-2222-222222222222', '33333333-3333-3333-3333-333333333333',
        'bbbbbbbb-0000-0000-0000-000000000002', 'PENDING', '¿Tapeo el viernes?', 72,
        NOW() + INTERVAL '70 hours', NULL, NOW() - INTERVAL '2 hours');

-- ── Reportes ─────────────────────────────────────────────
INSERT INTO reports (report_id, reporter_id, target_type, target_id, reason, comment, status, reviewed_at, reviewed_by) VALUES
    ('eeeeeeee-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333', 'USER',
        '66666666-6666-6666-6666-666666666666', 'HARASSMENT', 'Mensajes insistentes tras rechazar el plan.', 'PENDING', NULL, NULL),
    ('eeeeeeee-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111', 'PHOTO',
        'cccccccc-0000-0000-0000-000000000005', 'INAPPROPRIATE_CONTENT', 'Foto no corresponde al lugar.', 'DISMISSED',
        NOW() - INTERVAL '1 day', '99999999-9999-9999-9999-999999999999');

COMMIT;
