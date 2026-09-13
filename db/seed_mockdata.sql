-- ============================================================
-- Datos de prueba (mockup) para debugging, testing y demos
-- ============================================================
-- Uso:
--   docker compose exec -T postgres psql -U candela_user -d candela_db < db/seed_mockdata.sql
-- (o copiar/pegar en Adminer / psql)
--
-- Contraseña para TODOS los usuarios de prueba: "password123"
-- (hash bcrypt generado con bcrypt.DefaultCost)
-- ============================================================

BEGIN;

-- ── Limpieza de datos previos (orden estricto por dependencias FK) ─
DELETE FROM refresh_tokens;
DELETE FROM reports;
DELETE FROM plan_participants;
DELETE FROM plans;
DELETE FROM follows;
DELETE FROM check_ins;
DELETE FROM friendships;
DELETE FROM point_transactions;
DELETE FROM places;
DELETE FROM businesses;
DELETE FROM users;

-- ── 1. Usuarios ──────────────────────────────────────────────
-- password_hash corresponde a "password123"
-- Nota: La columna 'age' ya no existe; se calcula dinámicamente desde 'birth_date'.
INSERT INTO users (
    user_id, email, username, password_hash, birth_date,
    is_protected, account_status, privacy_mode,
    guardian_contact, guardian_verified, guardian_token,
    points_total, points_season, badge_level,
    streak_weeks_current, streak_weeks_max, streak_week_checkins, streak_week_start, streak_last_checkin,
    ranking_opt_in
) VALUES
    -- Ana: Activa, pública, en ranking global, racha activa
    ('11111111-1111-1111-1111-111111111111', 'ana.garcia@example.com', 'ana_garcia',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '1999-03-12',
     false, 'ACTIVE', 'PUBLIC',
     NULL, false, NULL,
     1240, 320, 'Parroquiano',
     3, 5, 2, CURRENT_DATE - INTERVAL '2 days', NOW() - INTERVAL '30 minutes',
     true),

    -- Carlos: Activo, amigos solamente, no en ranking global
    ('22222222-2222-2222-2222-222222222222', 'carlos.ruiz@example.com', 'carlos_ruiz',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2001-07-23',
     false, 'ACTIVE', 'FRIENDS',
     NULL, false, NULL,
     860, 210, 'Parroquiano',
     1, 2, 1, CURRENT_DATE - INTERVAL '1 day', NOW() - INTERVAL '45 minutes',
     false),

    -- Laura: Activa, pública, ranking activo
    ('33333333-3333-3333-3333-333333333333', 'laura.fernandez@example.com', 'laura_fdez',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2003-11-02',
     false, 'ACTIVE', 'PUBLIC',
     NULL, false, NULL,
     430, 120, 'Callejero',
     0, 1, 1, CURRENT_DATE, NOW() - INTERVAL '2 hours',
     true),

    -- Mateo: Menor de edad protegido (< 18), tutor verificado
    ('44444444-4444-4444-4444-444444444444', 'mateo.lopez@example.com', 'mateo_lopez',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2008-05-15',
     true, 'ACTIVE', 'FRIENDS',
     'padre.lopez@example.com', true, '44444444-0000-0000-0000-000000000001',
     95, 95, 'Nuevo en la vuelta',
     0, 0, 0, NULL, NULL,
     false),

    -- Sofía: Nivel alto, privada (aparece solo en ranking amigos)
    ('55555555-5555-5555-5555-555555555555', 'sofia.martinez@example.com', 'sofia_mtz',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '1997-09-30',
     false, 'ACTIVE', 'PRIVATE',
     NULL, false, NULL,
     2100, 540, 'Teso',
     6, 8, 2, CURRENT_DATE - INTERVAL '3 days', NOW() - INTERVAL '1 hour 30 minutes',
     true),

    -- Pedro: Registro pendiente de confirmación
    ('66666666-6666-6666-6666-666666666666', 'pedro.sanchez@example.com', 'pedro_sanchez',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2006-05-18',
     false, 'PENDING', 'FRIENDS',
     NULL, false, NULL,
     0, 0, 'Nuevo en la vuelta',
     0, 0, 0, NULL, NULL,
     false),

    -- Admin / Moderador
    ('99999999-9999-9999-9999-999999999999', 'admin@candela.example.com', 'admin',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '1990-01-01',
     false, 'ACTIVE', 'PRIVATE',
     NULL, false, NULL,
     0, 0, 'Nuevo en la vuelta',
     0, 0, 0, NULL, NULL,
     false);

-- ── 2. Negocios ──────────────────────────────────────────────
-- ENUMs migrados a VARCHAR + CHECK: 'FREE', 'BASIC', 'PREMIUM'
INSERT INTO businesses (business_id, legal_name, contact_email, plan, is_verified) VALUES
    ('aaaaaaaa-0000-0000-0000-000000000001', 'Cafetería La Plaza S.L.',   'contacto@laplaza.example.com',  'PREMIUM', true),
    ('aaaaaaaa-0000-0000-0000-000000000002', 'Bar El Rincón S.L.',        'info@elrincon.example.com',     'BASIC',   true),
    ('aaaaaaaa-0000-0000-0000-000000000003', 'Club Nocturno Aurora S.L.', 'reservas@aurora.example.com',   'FREE',    false);

-- ── 3. Lugares (coordenadas en Barranquilla) ─────────────────
-- location GEOGRAPHY(POINT, 4326) con ST_SetSRID(ST_MakePoint(lon, lat), 4326)
-- minor_suitability CHECK: 'YES', 'NO', 'PENDING'
-- category CHECK: 'RESTAURANT', 'CAFE', 'BAR', 'NIGHTCLUB', 'PARK', 'STORE', 'UNIVERSITY', 'EVENT', 'OTHER'
INSERT INTO places (
    place_id, name, description, category, minor_suitability,
    location, geofence_meters, business_id, is_verified, is_active
) VALUES
    ('bbbbbbbb-0000-0000-0000-000000000001', 'Cafetería La Plaza',
     'Cafetería de especialidad en pleno centro, ideal para estudiar o parchar.',
     'CAFE', 'YES',
     ST_SetSRID(ST_MakePoint(-74.8080, 11.0110), 4326)::geography, 80,
     'aaaaaaaa-0000-0000-0000-000000000001', true, true),

    ('bbbbbbbb-0000-0000-0000-000000000002', 'Bar El Rincón',
     'Bar con terraza, cocteles y música en vivo los fines de semana.',
     'BAR', 'NO',
     ST_SetSRID(ST_MakePoint(-74.8200, 11.0220), 4326)::geography, 100,
     'aaaaaaaa-0000-0000-0000-000000000002', true, true),

    ('bbbbbbbb-0000-0000-0000-000000000003', 'Club Nocturno Aurora',
     'Discoteca de música electrónica y eventos nocturnos en la zona rosa.',
     'NIGHTCLUB', 'NO',
     ST_SetSRID(ST_MakePoint(-74.8050, 11.0250), 4326)::geography, 150,
     'aaaaaaaa-0000-0000-0000-000000000003', false, true),

    ('bbbbbbbb-0000-0000-0000-000000000004', 'Parque Cultural del Caribe',
     'Gran parque urbano con zonas verdes, perfecto para planes al aire libre.',
     'PARK', 'YES',
     ST_SetSRID(ST_MakePoint(-74.7990, 11.0040), 4326)::geography, 200,
     NULL, true, true),

    ('bbbbbbbb-0000-0000-0000-000000000005', 'Universidad del Norte',
     'Campus universitario con zonas comunes, plazoletas de comidas y biblioteca.',
     'UNIVERSITY', 'YES',
     ST_SetSRID(ST_MakePoint(-74.8510, 11.0350), 4326)::geography, 200,
     NULL, true, true),

    ('bbbbbbbb-0000-0000-0000-000000000006', 'Restaurante Sabor Caribe',
     'Cocina de mercado con pescados frescos y ambiente acogedor.',
     'RESTAURANT', 'YES',
     ST_SetSRID(ST_MakePoint(-74.8120, 11.0200), 4326)::geography, 100,
     NULL, false, true);

-- ── 4. Check-ins ─────────────────────────────────────────────
-- location GEOGRAPHY(POINT, 4326)
-- Con fotos de prueba y soporte para testing de janitor/expiración
INSERT INTO check_ins (
    checkin_id, user_id, place_id,
    photo_url, photo_key, photo_expires_at, photo_exif_ts, photo_is_permanent,
    location, gps_valid, timestamp_valid, validation_passed,
    points_awarded, is_first_of_day, checked_in_at
) VALUES
    -- Ana en Cafetería La Plaza (foto permanente, check-in activo hace 30m)
    ('cccccccc-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000001',
     'http://localhost:9000/candela-photos/checkin_01.jpg', 'checkin_01.jpg', NULL, NOW() - INTERVAL '30 minutes', true,
     ST_SetSRID(ST_MakePoint(-74.8080, 11.0110), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '30 minutes'),

    -- Ana en Parque Cultural (hace 1h, foto temporal con expiración a 24h)
    ('cccccccc-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000004',
     'http://localhost:9000/candela-photos/checkin_02.jpg', 'checkin_02.jpg', NOW() + INTERVAL '23 hours', NOW() - INTERVAL '1 hour', false,
     ST_SetSRID(ST_MakePoint(-74.7990, 11.0040), 4326)::geography, true, true, true,
     15, false, NOW() - INTERVAL '1 hour'),

    -- Carlos en Bar El Rincón (hace 45m)
    ('cccccccc-0000-0000-0000-000000000003', '22222222-2222-2222-2222-222222222222', 'bbbbbbbb-0000-0000-0000-000000000002',
     'http://localhost:9000/candela-photos/checkin_03.jpg', 'checkin_03.jpg', NOW() + INTERVAL '23 hours', NOW() - INTERVAL '45 minutes', false,
     ST_SetSRID(ST_MakePoint(-74.8200, 11.0220), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '45 minutes'),

    -- Laura en Restaurante Sabor Caribe (hace 2h)
    ('cccccccc-0000-0000-0000-000000000004', '33333333-3333-3333-3333-333333333333', 'bbbbbbbb-0000-0000-0000-000000000006',
     'http://localhost:9000/candela-photos/checkin_04.jpg', 'checkin_04.jpg', NOW() + INTERVAL '22 hours', NOW() - INTERVAL '2 hours', false,
     ST_SetSRID(ST_MakePoint(-74.8120, 11.0200), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '2 hours'),

    -- Sofía en Club Nocturno Aurora (hace 1h 30m, reportada por foto)
    ('cccccccc-0000-0000-0000-000000000005', '55555555-5555-5555-5555-555555555555', 'bbbbbbbb-0000-0000-0000-000000000003',
     'http://localhost:9000/candela-photos/checkin_05.jpg', 'checkin_05.jpg', NOW() + INTERVAL '22 hours', NOW() - INTERVAL '1 hour 30 minutes', false,
     ST_SetSRID(ST_MakePoint(-74.8050, 11.0250), 4326)::geography, true, true, true,
     25, true, NOW() - INTERVAL '1 hour 30 minutes'),

    -- Sofía en Cafetería La Plaza (hace 3h)
    ('cccccccc-0000-0000-0000-000000000006', '55555555-5555-5555-5555-555555555555', 'bbbbbbbb-0000-0000-0000-000000000001',
     'http://localhost:9000/candela-photos/checkin_06.jpg', 'checkin_06.jpg', NOW() + INTERVAL '21 hours', NOW() - INTERVAL '3 hours', false,
     ST_SetSRID(ST_MakePoint(-74.8080, 11.0110), 4326)::geography, true, true, true,
     20, false, NOW() - INTERVAL '3 hours'),

    -- Check-in expirado (hace 2 días, foto expirada) útil para probar worker janitor
    ('cccccccc-0000-0000-0000-000000000007', '11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000005',
     'http://localhost:9000/candela-photos/checkin_expired.jpg', 'checkin_expired.jpg', NOW() - INTERVAL '1 day', NOW() - INTERVAL '2 days', false,
     ST_SetSRID(ST_MakePoint(-74.8510, 11.0350), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '2 days');

-- ── 5. Transacciones de Puntos ───────────────────────────────
-- reason CHECK: 'CHECKIN', 'PLAN', 'REFERRAL'
INSERT INTO point_transactions (tx_id, user_id, points, reason, reference_id, created_at) VALUES
    ('ffffffff-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 20, 'CHECKIN',  'cccccccc-0000-0000-0000-000000000001', NOW() - INTERVAL '30 minutes'),
    ('ffffffff-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111', 15, 'CHECKIN',  'cccccccc-0000-0000-0000-000000000002', NOW() - INTERVAL '1 hour'),
    ('ffffffff-0000-0000-0000-000000000003', '22222222-2222-2222-2222-222222222222', 20, 'CHECKIN',  'cccccccc-0000-0000-0000-000000000003', NOW() - INTERVAL '45 minutes'),
    ('ffffffff-0000-0000-0000-000000000004', '33333333-3333-3333-3333-333333333333', 20, 'CHECKIN',  'cccccccc-0000-0000-0000-000000000004', NOW() - INTERVAL '2 hours'),
    ('ffffffff-0000-0000-0000-000000000005', '55555555-5555-5555-5555-555555555555', 25, 'CHECKIN',  'cccccccc-0000-0000-0000-000000000005', NOW() - INTERVAL '1 hour 30 minutes'),
    ('ffffffff-0000-0000-0000-000000000006', '55555555-5555-5555-5555-555555555555', 50, 'PLAN',     'dddddddd-0000-0000-0000-000000000002', NOW() - INTERVAL '2 days'),
    ('ffffffff-0000-0000-0000-000000000007', '11111111-1111-1111-1111-111111111111', 50, 'PLAN',     'dddddddd-0000-0000-0000-000000000002', NOW() - INTERVAL '2 days'),
    ('ffffffff-0000-0000-0000-000000000008', '11111111-1111-1111-1111-111111111111', 100, 'REFERRAL', '22222222-2222-2222-2222-222222222222', NOW() - INTERVAL '15 days');

-- ── 6. Follows (usuarios siguiendo lugares) ──────────────────
INSERT INTO follows (follower_id, place_id) VALUES
    ('11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000001'),
    ('11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000003'),
    ('22222222-2222-2222-2222-222222222222', 'bbbbbbbb-0000-0000-0000-000000000002'),
    ('33333333-3333-3333-3333-333333333333', 'bbbbbbbb-0000-0000-0000-000000000004'),
    ('55555555-5555-5555-5555-555555555555', 'bbbbbbbb-0000-0000-0000-000000000001'),
    ('55555555-5555-5555-5555-555555555555', 'bbbbbbbb-0000-0000-0000-000000000006');

-- ── 7. Amistades (user_id_a < user_id_b garantizado) ─────────
-- status CHECK: 'pending', 'accepted', 'blocked'
INSERT INTO friendships (user_id_a, user_id_b, requester_id, status, created_at) VALUES
    ('11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111', 'accepted', NOW() - INTERVAL '30 days'),
    ('11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555555', '55555555-5555-5555-5555-555555555555', 'accepted', NOW() - INTERVAL '20 days'),
    ('22222222-2222-2222-2222-222222222222', '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222222', 'pending',  NOW() - INTERVAL '2 days'),
    ('33333333-3333-3333-3333-333333333333', '55555555-5555-5555-5555-555555555555', '33333333-3333-3333-3333-333333333333', 'pending',  NOW() - INTERVAL '1 day'),
    ('22222222-2222-2222-2222-222222222222', '66666666-6666-6666-6666-666666666666', '22222222-2222-2222-2222-222222222222', 'blocked',  NOW() - INTERVAL '5 days');

-- ── 8. Planes ────────────────────────────────────────────────
-- status CHECK: 'PENDING', 'ACCEPTED', 'COMPLETED', 'FAILED', 'DECLINED'
-- visibility CHECK: 'PUBLIC', 'FRIENDS', 'INVITE'
-- deadline_hours CHECK: 24, 48, 72
INSERT INTO plans (
    plan_id, creator_user_id, target_user_id, place_id,
    status, visibility, message,
    deadline_hours, deadline_at, completed_at, created_at
) VALUES
    -- Plan 1: Directo aceptado entre Ana y Carlos
    ('dddddddd-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222',
     'bbbbbbbb-0000-0000-0000-000000000001',
     'ACCEPTED', 'INVITE', '¿Café mañana por la tarde?',
     24, NOW() + INTERVAL '20 hours', NULL, NOW() - INTERVAL '4 hours'),

    -- Plan 2: Completado exitosamente entre Sofía y Ana
    ('dddddddd-0000-0000-0000-000000000002', '55555555-5555-5555-5555-555555555555', '11111111-1111-1111-1111-111111111111',
     'bbbbbbbb-0000-0000-0000-000000000004',
     'COMPLETED', 'FRIENDS', 'Paseo por el Parque Cultural este finde',
     48, NOW() - INTERVAL '1 day', NOW() - INTERVAL '2 days', NOW() - INTERVAL '5 days'),

    -- Plan 3: Directo pendiente de Carlos hacia Laura
    ('dddddddd-0000-0000-0000-000000000003', '22222222-2222-2222-2222-222222222222', '33333333-3333-3333-3333-333333333333',
     'bbbbbbbb-0000-0000-0000-000000000002',
     'PENDING', 'INVITE', '¿Tapeo el viernes?',
     72, NOW() + INTERVAL '70 hours', NULL, NOW() - INTERVAL '2 hours'),

    -- Plan 4: Plan ABIERTO (Mesa Nómade / Meetup público sin target_user_id)
    -- Permite probar el endpoint /api/plans/discover
    ('dddddddd-0000-0000-0000-000000000004', '11111111-1111-1111-1111-111111111111', NULL,
     'bbbbbbbb-0000-0000-0000-000000000001',
     'PENDING', 'PUBLIC', 'Mesa de coworking y charla sobre startups. ¡Caigan!',
     48, NOW() + INTERVAL '40 hours', NULL, NOW() - INTERVAL '1 hour');

-- ── 9. Participantes de Planes ───────────────────────────────
-- role CHECK: 'CREATOR', 'GUEST'
-- status CHECK: 'INVITED', 'JOINED', 'COMPLETED', 'DECLINED'
INSERT INTO plan_participants (plan_id, user_id, role, status, joined_at, completed_at) VALUES
    -- Plan 1 (Ana y Carlos - Aceptado)
    ('dddddddd-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'CREATOR', 'JOINED', NOW() - INTERVAL '4 hours', NULL),
    ('dddddddd-0000-0000-0000-000000000001', '22222222-2222-2222-2222-222222222222', 'GUEST',   'JOINED', NOW() - INTERVAL '3 hours', NULL),

    -- Plan 2 (Sofía y Ana - Completado)
    ('dddddddd-0000-0000-0000-000000000002', '55555555-5555-5555-5555-555555555555', 'CREATOR', 'COMPLETED', NOW() - INTERVAL '5 days', NOW() - INTERVAL '2 days'),
    ('dddddddd-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111', 'GUEST',   'COMPLETED', NOW() - INTERVAL '4 days', NOW() - INTERVAL '2 days'),

    -- Plan 3 (Carlos hacia Laura - Pendiente)
    ('dddddddd-0000-0000-0000-000000000003', '22222222-2222-2222-2222-222222222222', 'CREATOR', 'JOINED',  NOW() - INTERVAL '2 hours', NULL),
    ('dddddddd-0000-0000-0000-000000000003', '33333333-3333-3333-3333-333333333333', 'GUEST',   'INVITED', NOW() - INTERVAL '2 hours', NULL),

    -- Plan 4 (Mesa abierta de Ana - Laura ya se unió)
    ('dddddddd-0000-0000-0000-000000000004', '11111111-1111-1111-1111-111111111111', 'CREATOR', 'JOINED', NOW() - INTERVAL '1 hour', NULL),
    ('dddddddd-0000-0000-0000-000000000004', '33333333-3333-3333-3333-333333333333', 'GUEST',   'JOINED', NOW() - INTERVAL '30 minutes', NULL);

-- ── 10. Reportes ─────────────────────────────────────────────
-- target_type CHECK: 'PHOTO', 'USER', 'PLACE'
-- reason CHECK: 'INAPPROPRIATE_CONTENT', 'HARASSMENT', 'SPAM', 'FAKE_CONTENT', 'OTHER'
-- status CHECK: 'PENDING', 'REVIEWED', 'DISMISSED'
INSERT INTO reports (
    report_id, reporter_id, target_type, target_id,
    reason, comment, status, reviewed_at, reviewed_by
) VALUES
    -- Reporte a usuario (Pendiente)
    ('eeeeeeee-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333',
     'USER', '66666666-6666-6666-6666-666666666666',
     'HARASSMENT', 'Mensajes insistentes e inapropiados tras rechazar invitación.',
     'PENDING', NULL, NULL),

    -- Reporte a foto (Desestimado por Admin)
    ('eeeeeeee-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111',
     'PHOTO', 'cccccccc-0000-0000-0000-000000000005',
     'INAPPROPRIATE_CONTENT', 'Foto no corresponde con el interior del local.',
     'DISMISSED', NOW() - INTERVAL '1 day', '99999999-9999-9999-9999-999999999999'),

    -- Reporte a lugar (Revisado por Admin)
    ('eeeeeeee-0000-0000-0000-000000000003', '22222222-2222-2222-2222-222222222222',
     'PLACE', 'bbbbbbbb-0000-0000-0000-000000000003',
     'SPAM', 'Nombre duplicado y geolocalización ligeramente desplazada.',
     'REVIEWED', NOW() - INTERVAL '12 hours', '99999999-9999-9999-9999-999999999999');

-- ── 11. Refresh Tokens (Sesiones activas) ─────────────────────
-- token_hash almacena bytea (hash SHA256 simulado)
INSERT INTO refresh_tokens (token_id, user_id, token_hash, expires_at, revoked_at) VALUES
    ('77777777-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111',
     decode('e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855', 'hex'),
     NOW() + INTERVAL '30 days', NULL);

COMMIT;
