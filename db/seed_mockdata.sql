-- ============================================================================
-- CANDELA API — DATOS DE PRUEBA (MOCK DATA) PARA DESARROLLO Y TESTING
-- ============================================================================
-- Ciudad base: Barranquilla, Colombia (Zona horaria: America/Bogota)
-- Contraseña para TODOS los usuarios de prueba: "password123"
-- (hash bcrypt estándar: $2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW)
--
-- Uso:
--   make seed
--   (o: docker compose exec -T postgres psql -U candela_user -d candela_db < db/seed_mockdata.sql)
-- ============================================================================

BEGIN;

-- ── 0. Limpieza profunda en orden estricto de dependencias FK ──────────────
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

-- ── 1. Usuarios (14 perfiles representativos de toda la app) ───────────────
-- Cubren: Todos los rangos (Nuevo en la vuelta, Callejero, Parroquiano, Teso, Leyenda)
-- Estados: ACTIVE, PENDING, SUSPENDED
-- Privacidad: PUBLIC (en ranking global), FRIENDS (amigos solamente), PRIVATE (oculto)
-- Menor de edad protegido (<18, guardian verificado)
INSERT INTO users (
    user_id, email, username, password_hash, birth_date,
    is_protected, account_status, privacy_mode,
    guardian_contact, guardian_verified, guardian_token,
    points_total, points_season, badge_level,
    streak_weeks_current, streak_weeks_max, streak_week_checkins, streak_week_start, streak_last_checkin,
    ranking_opt_in
) VALUES
    -- 1. Sebastián: Top #1 de Barranquilla (Rango "Leyenda"), Podio Oro
    ('10101010-1010-1010-1010-101010101010', 'sebastian@example.com', 'el_rey_del_parche',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '1995-08-14',
     false, 'ACTIVE', 'PUBLIC',
     NULL, false, NULL,
     5420, 1280, 'Leyenda',
     12, 16, 4, CURRENT_DATE - INTERVAL '1 day', NOW() - INTERVAL '30 minutes',
     true),

    -- 2. Valentina: Top #2 de la ciudad (Rango "Teso"), Podio Plata, amiga de Ana
    ('20202020-2020-2020-2020-202020202020', 'valentina@example.com', 'valen_curramba',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2000-02-18',
     false, 'ACTIVE', 'PUBLIC',
     NULL, false, NULL,
     3450, 890, 'Teso',
     7, 10, 3, CURRENT_DATE - INTERVAL '2 days', NOW() - INTERVAL '20 minutes',
     true),

    -- 3. Andrés: Top #3 de la ciudad (Rango "Teso"), Podio Bronce, no amigo
    ('30303030-3030-3030-3030-303030303030', 'andres.mendoza@example.com', 'andres_mendoza',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '1998-11-05',
     false, 'ACTIVE', 'PUBLIC',
     NULL, false, NULL,
     1750, 480, 'Teso',
     4, 6, 2, CURRENT_DATE - INTERVAL '2 days', NOW() - INTERVAL '1 hour 15 minutes',
     true),

    -- 4. Ana García: USUARIO PRINCIPAL DE TESTING (Rango "Parroquiano", Puesto #4)
    ('11111111-1111-1111-1111-111111111111', 'ana.garcia@example.com', 'ana_garcia',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '1999-03-12',
     false, 'ACTIVE', 'PUBLIC',
     NULL, false, NULL,
     1240, 320, 'Parroquiano',
     3, 5, 2, CURRENT_DATE - INTERVAL '2 days', NOW() - INTERVAL '35 minutes',
     true),

    -- 5. Carlos: Amigo de Ana, perfil amigos, no en ranking global (opt-out)
    ('22222222-2222-2222-2222-222222222222', 'carlos.ruiz@example.com', 'carlos_ruiz',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2001-07-23',
     false, 'ACTIVE', 'FRIENDS',
     NULL, false, NULL,
     860, 210, 'Parroquiano',
     2, 4, 1, CURRENT_DATE - INTERVAL '1 day', NOW() - INTERVAL '1 hour',
     false),

    -- 6. Mariana: Puesto #5 ranking global, no amiga de Ana
    ('40404040-4040-4040-4040-404040404040', 'mariana.costa@example.com', 'mariana_costa',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2001-04-10',
     false, 'ACTIVE', 'PUBLIC',
     NULL, false, NULL,
     680, 190, 'Parroquiano',
     2, 3, 1, CURRENT_DATE - INTERVAL '3 days', NOW() - INTERVAL '2 hours',
     true),

    -- 7. Laura: Rango "Callejero", solicitud enviada pendiente por Ana
    ('33333333-3333-3333-3333-333333333333', 'laura.fernandez@example.com', 'laura_fdez',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2003-11-02',
     false, 'ACTIVE', 'PUBLIC',
     NULL, false, NULL,
     430, 120, 'Callejero',
     1, 2, 1, CURRENT_DATE, NOW() - INTERVAL '1 hour 30 minutes',
     true),

    -- 8. Diego: Rango "Callejero", público, prueba actividad ciudad
    ('50505050-5050-5050-5050-505050505050', 'diego.marin@example.com', 'diego_marin',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2002-09-17',
     false, 'ACTIVE', 'PUBLIC',
     NULL, false, NULL,
     260, 80, 'Callejero',
     1, 1, 1, CURRENT_DATE - INTERVAL '1 day', NOW() - INTERVAL '2 hours 45 minutes',
     true),

    -- 9. Mateo: MENOR DE EDAD PROTEGIDO (<18), tutor verificado, no apto para discotecas
    ('44444444-4444-4444-4444-444444444444', 'mateo.lopez@example.com', 'mateo_lopez',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2008-05-15',
     true, 'ACTIVE', 'FRIENDS',
     'padre.lopez@example.com', true, '44444444-0000-0000-0000-000000000001',
     95, 95, 'Nuevo en la vuelta',
     0, 0, 0, NULL, NULL,
     false),

    -- 10. Camilo: Rango "Nuevo en la vuelta", SOLICITUD DE AMISTAD ENTRANTE PARA ANA
    ('60606060-6060-6060-6060-606060606060', 'camilo.vega@example.com', 'camilo_vega',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2005-01-20',
     false, 'ACTIVE', 'PUBLIC',
     NULL, false, NULL,
     45, 45, 'Nuevo en la vuelta',
     0, 0, 0, NULL, NULL,
     true),

    -- 11. Sofía: Perfil PRIVADO (amigos ven perfil, extraños reciben 404)
    ('55555555-5555-5555-5555-555555555555', 'sofia.martinez@example.com', 'sofia_mtz',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '1997-09-30',
     false, 'ACTIVE', 'PRIVATE',
     NULL, false, NULL,
     2100, 540, 'Teso',
     6, 8, 2, CURRENT_DATE - INTERVAL '3 days', NOW() - INTERVAL '2 hours 15 minutes',
     true),

    -- 12. Pedro: Registro pendiente de verificación por email
    ('66666666-6666-6666-6666-666666666666', 'pedro.sanchez@example.com', 'pedro_sanchez',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '2004-06-12',
     false, 'PENDING', 'FRIENDS',
     NULL, false, NULL,
     0, 0, 'Nuevo en la vuelta',
     0, 0, 0, NULL, NULL,
     false),

    -- 13. Esteban: CUENTA SUSPENDIDA (Prueba el manejo de 403 Forbidden en el front)
    ('70707070-7070-7070-7070-707070707070', 'esteban@example.com', 'esteban_bloqueado',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '1996-12-05',
     false, 'SUSPENDED', 'FRIENDS',
     NULL, false, NULL,
     120, 0, 'Nuevo en la vuelta',
     0, 1, 0, NULL, NULL,
     false),

    -- 14. Administrador / Moderador
    ('99999999-9999-9999-9999-999999999999', 'admin@candela.example.com', 'admin',
     '$2a$10$eUY8NKUlQg6/065ZWE2/NuLFR6JegcGKztjY9ODdd/6el6i7gRVmW', '1990-01-01',
     false, 'ACTIVE', 'PRIVATE',
     NULL, false, NULL,
     0, 0, 'Nuevo en la vuelta',
     0, 0, 0, NULL, NULL,
     false);

-- ── 2. Negocios Comerciales de Barranquilla ────────────────────────────────
-- Planes de suscripción comercial: FREE, BASIC, PREMIUM
INSERT INTO businesses (business_id, legal_name, contact_email, plan, is_verified) VALUES
    ('aaaaaaaa-0000-0000-0000-000000000001', 'La Troja S.A.S.',                     'administracion@latroja.org',        'PREMIUM', true),
    ('aaaaaaaa-0000-0000-0000-000000000002', 'Caimán del Río Gastronomía S.A.S.',   'eventos@caimandelrio.com',          'PREMIUM', true),
    ('aaaaaaaa-0000-0000-0000-000000000003', 'Grupo Díscolo Nightclub S.A.S.',      'reservas@discolo.com.co',           'BASIC',   true),
    ('aaaaaaaa-0000-0000-0000-000000000004', 'Café San Alberto Barranquilla S.A.S.','barranquilla@cafesanalberto.com',  'BASIC',   true),
    ('aaaaaaaa-0000-0000-0000-000000000005', 'Narcobollo Tradición Costeña S.A.S.',  'pedidos@narcobollo.com',            'FREE',    true),
    ('aaaaaaaa-0000-0000-0000-000000000006', 'Frogg Club Barranquilla S.A.S.',       'info@froggclub.com',                'FREE',    false);

-- ── 3. Lugares Reales de Barranquilla (18 Spots Calibrados) ────────────────
-- Coordenadas reales con PostGIS ST_SetSRID(ST_MakePoint(lon, lat), 4326)
-- Categorías: BAR, NIGHTCLUB, RESTAURANT, CAFE, PARK, UNIVERSITY, STORE, EVENT
-- minor_suitability: YES, NO, PENDING
-- geofence_meters: entre 20 y 150 metros
INSERT INTO places (
    place_id, name, description, category, minor_suitability,
    location, geofence_meters, business_id, is_verified, is_active
) VALUES
    -- ── BARES ──
    -- La Troja (El ícono de la salsa, Patrimonio Cultural y Musical de Barranquilla)
    ('bbbbbbbb-0000-0000-0000-000000000001', 'La Troja (Patrimonio Cultural)',
     'La esquina más salsera de Colombia. Baile en la calle, cerveza fría y salsa brava tradicional.',
     'BAR', 'NO',
     ST_SetSRID(ST_MakePoint(-74.8075, 10.9982), 4326)::geography, 100,
     'aaaaaaaa-0000-0000-0000-000000000001', true, true),

    -- Bourbon St. Bar (Pub estilo Nueva Orleans en El Golf)
    ('bbbbbbbb-0000-0000-0000-000000000002', 'Bourbon St. Bar',
     'Pub temático con ambiente de Nueva Orleans, cocteles de autor y rock/blues en vivo los fines de semana.',
     'BAR', 'NO',
     ST_SetSRID(ST_MakePoint(-74.8115, 11.0065), 4326)::geography, 80,
     NULL, true, true),

    -- El Callejón (Barrio Abajo, cerca del centro)
    ('bbbbbbbb-0000-0000-0000-000000000003', 'El Callejón Barrio Abajo',
     'Bar bohemio en pleno corazón de Barrio Abajo. Coctelería con frutas tropicales y ambiente relajado.',
     'BAR', 'NO',
     ST_SetSRID(ST_MakePoint(-74.7865, 10.9925), 4326)::geography, 80,
     NULL, false, true),

    -- ── RUMBA / NIGHTCLUBS ──
    -- Díscolo (Discoteca de alta gama en CC Miramar)
    ('bbbbbbbb-0000-0000-0000-000000000004', 'Díscolo Nightclub',
     'Discoteca líder en la rumba currambera. Luces láser de última tecnología, reggaetón, electrónica y shows en vivo.',
     'NIGHTCLUB', 'NO',
     ST_SetSRID(ST_MakePoint(-74.8235, 11.0118), 4326)::geography, 120,
     'aaaaaaaa-0000-0000-0000-000000000003', true, true),

    -- Frogg Club (Zona Rosa de la 84)
    ('bbbbbbbb-0000-0000-0000-000000000005', 'Frogg Club',
     'Club nocturno crossover sobre la Calle 84. El epicentro del desorden y la fiesta universitaria.',
     'NIGHTCLUB', 'NO',
     ST_SetSRID(ST_MakePoint(-74.8155, 11.0105), 4326)::geography, 120,
     'aaaaaaaa-0000-0000-0000-000000000006', false, true),

    -- Club Nocturno Aurora (Calle 82)
    ('bbbbbbbb-0000-0000-0000-000000000006', 'Club Nocturno Aurora',
     'Club de música electrónica y formato afterhours en el norte de Barranquilla.',
     'NIGHTCLUB', 'NO',
     ST_SetSRID(ST_MakePoint(-74.8130, 11.0080), 4326)::geography, 120,
     NULL, false, true),

    -- ── RESTAURANTES / GASTRONOMÍA ──
    -- Caimán del Río (Gran Malecón del Río)
    ('bbbbbbbb-0000-0000-0000-000000000007', 'Caimán del Río (Mercado Gastronómico)',
     'Mercado gastronómico de 25 cocinas a orillas del Río Magdalena. Marisco fresco, carnes y vista inigualable.',
     'RESTAURANT', 'YES',
     ST_SetSRID(ST_MakePoint(-74.7890, 11.0125), 4326)::geography, 150,
     'aaaaaaaa-0000-0000-0000-000000000002', true, true),

    -- Narcobollo (Tradición gastronómica costeña)
    ('bbbbbbbb-0000-0000-0000-000000000008', 'Narcobollo Tradición Costeña',
     'El templo de la comida típica barranquillera: bollos de yuca, butifarra, caribañolas y chicharrón crocante.',
     'RESTAURANT', 'YES',
     ST_SetSRID(ST_MakePoint(-74.8210, 11.0110), 4326)::geography, 80,
     'aaaaaaaa-0000-0000-0000-000000000005', true, true),

    -- Cucayo (Sabor caribeño auténtico)
    ('bbbbbbbb-0000-0000-0000-000000000009', 'Restaurante Cucayo',
     'Homenaje a la sazón caribeña servido en totumas y cazuelas de barro. Arroz con coco y posta negra.',
     'RESTAURANT', 'YES',
     ST_SetSRID(ST_MakePoint(-74.8100, 11.0040), 4326)::geography, 80,
     NULL, true, true),

    -- Miura Steakhouse (Cortes premium en El Golf)
    ('bbbbbbbb-0000-0000-0000-000000000010', 'Miura Steakhouse',
     'Cortes de carne madurados, coctelería clásica y cava de vinos en un ambiente contemporáneo.',
     'RESTAURANT', 'YES',
     ST_SetSRID(ST_MakePoint(-74.8125, 11.0060), 4326)::geography, 90,
     NULL, true, true),

    -- ── CAFETERÍAS ──
    -- Café San Alberto (Café de especialidad premium)
    ('bbbbbbbb-0000-0000-0000-000000000011', 'Café San Alberto (Especialidad)',
     'El café más premiado de Colombia. Filtrados V60, espresso bar y repostería artesanal.',
     'CAFE', 'YES',
     ST_SetSRID(ST_MakePoint(-74.8090, 11.0035), 4326)::geography, 70,
     'aaaaaaaa-0000-0000-0000-000000000004', true, true),

    -- Juan Valdez - Terraza Alto Prado
    ('bbbbbbbb-0000-0000-0000-000000000012', 'Juan Valdez - Alto Prado',
     'Amplia terraza sombreada sobre la Cra 53. Punto de encuentro clásico para café helado y coworking.',
     'CAFE', 'YES',
     ST_SetSRID(ST_MakePoint(-74.8120, 11.0070), 4326)::geography, 70,
     NULL, true, true),

    -- Café de la Plaza (Plaza de la Paz - Centro)
    ('bbbbbbbb-0000-0000-0000-000000000013', 'Café de la Plaza de la Paz',
     'Café al aire libre junto a la Catedral María Reina. Gran vista de la plaza y bebidas refrescantes.',
     'CAFE', 'YES',
     ST_SetSRID(ST_MakePoint(-74.7935, 10.9890), 4326)::geography, 80,
     NULL, false, true),

    -- ── PARQUES Y ESPACIOS AL AIRE LIBRE ──
    -- Gran Malecón del Río
    ('bbbbbbbb-0000-0000-0000-000000000014', 'Gran Malecón del Río',
     'El bulevar peatonal más visitado de Colombia. 5 km de sendero frente al Río Magdalena, brisa fresca y ciclorruta.',
     'PARK', 'YES',
     ST_SetSRID(ST_MakePoint(-74.7850, 11.0080), 4326)::geography, 150,
     NULL, true, true),

    -- Plaza de la Paz (Cerca del fallbackLat del front)
    ('bbbbbbbb-0000-0000-0000-000000000015', 'Plaza de la Paz San Juan Pablo II',
     'Gran plaza cívica de la ciudad. Punto de encuentro para skaters, caminantes y eventos públicos al aire libre.',
     'PARK', 'YES',
     ST_SetSRID(ST_MakePoint(-74.7930, 10.9895), 4326)::geography, 150,
     NULL, true, true),

    -- Parque Cultural del Caribe (Centro / Barrio Abajo)
    ('bbbbbbbb-0000-0000-0000-000000000016', 'Parque Cultural del Caribe',
     'Parque urbano y complejo cultural con jardines nativos, fuentes y zonas de descanso sombreadas.',
     'PARK', 'YES',
     ST_SetSRID(ST_MakePoint(-74.7810, 10.9855), 4326)::geography, 120,
     NULL, true, true),

    -- ── UNIVERSIDADES ──
    -- Universidad del Norte (Uninorte)
    ('bbbbbbbb-0000-0000-0000-000000000017', 'Universidad del Norte (Uninorte)',
     'Campus verde con plazoletas de comidas, lago de iguanas y biblioteca Karl C. Parrish. Epicentro universitario.',
     'UNIVERSITY', 'YES',
     ST_SetSRID(ST_MakePoint(-74.8505, 11.0195), 4326)::geography, 150,
     NULL, true, true),

    -- Universidad Simón Bolívar (Sede Prado)
    ('bbbbbbbb-0000-0000-0000-000000000018', 'Universidad Simón Bolívar',
     'Casonas patrimoniales de estilo republicano y plazoletas estudiantiles en el histórico Barrio El Prado.',
     'UNIVERSITY', 'YES',
     ST_SetSRID(ST_MakePoint(-74.7950, 10.9995), 4326)::geography, 150,
     NULL, true, true),

    -- ── COMERCIO / SHOPPING MALLS ──
    -- Centro Comercial Viva Barranquilla
    ('bbbbbbbb-0000-0000-0000-000000000019', 'C.C. Viva Barranquilla',
     'El centro comercial más grande de la Costa Caribe. Plazas de comida, terrazas gastronómicas y cine.',
     'STORE', 'YES',
     ST_SetSRID(ST_MakePoint(-74.8220, 11.0120), 4326)::geography, 150,
     NULL, true, true),

    -- Centro Comercial Buenavista
    ('bbbbbbbb-0000-0000-0000-000000000020', 'C.C. Buenavista',
     'Punto neurálgico del norte de Barranquilla con tiendas de moda, cafés y restaurantes internacionales.',
     'STORE', 'YES',
     ST_SetSRID(ST_MakePoint(-74.8280, 11.0190), 4326)::geography, 150,
     NULL, true, true),

    -- ── EVENTOS ──
    -- Puerta de Oro (Centro de Eventos del Caribe)
    ('bbbbbbbb-0000-0000-0000-000000000021', 'Puerta de Oro (Centro de Eventos)',
     'Gran recinto ferial junto al río. Sede de ferias gastronómicas Sabor Barranquilla, conciertos y exposiciones.',
     'EVENT', 'YES',
     ST_SetSRID(ST_MakePoint(-74.7950, 11.0160), 4326)::geography, 150,
     NULL, true, true);

-- ── 4. Check-Ins Recientes e Históricos ────────────────────────────────────
-- Las fotos activas usan URLs de alta resolución de Unsplash (temática de cafés,
-- rumbas, gastronomía costeña y paisajes).
-- Check-ins en las últimas 4 horas activan la "llama" (is_active) en los lugares
-- y alimentan las 3 capas del Feed Social de Ana.
INSERT INTO check_ins (
    checkin_id, user_id, place_id,
    photo_url, photo_key, photo_expires_at, photo_exif_ts, photo_is_permanent,
    location, gps_valid, timestamp_valid, validation_passed,
    points_awarded, is_first_of_day, checked_in_at
) VALUES
    -- 1. Ana en Café San Alberto (hace 35 min, FOTO PERMANENTE, primera del día -> Capa 1 Feed de Ana)
    ('cccccccc-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000011',
     'https://images.unsplash.com/photo-1501339847302-ac426a4a7cbb?w=900&auto=format&fit=crop&q=80', 'checkins/11111111-1111-1111-1111-111111111111/cafe_sanalberto.jpg',
     NULL, NOW() - INTERVAL '35 minutes', true,
     ST_SetSRID(ST_MakePoint(-74.8090, 11.0035), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '35 minutes'),

    -- 2. Valentina (Amiga de Ana) en Caimán del Río (hace 20 min -> Capa 1 Feed de Ana)
    ('cccccccc-0000-0000-0000-000000000002', '20202020-2020-2020-2020-202020202020', 'bbbbbbbb-0000-0000-0000-000000000007',
     'https://images.unsplash.com/photo-1555396273-367ea4eb4db5?w=900&auto=format&fit=crop&q=80', 'checkins/20202020-2020-2020-2020-202020202020/caiman_valen.jpg',
     NOW() + INTERVAL '23 hours', NOW() - INTERVAL '20 minutes', false,
     ST_SetSRID(ST_MakePoint(-74.7890, 11.0125), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '20 minutes'),

    -- 3. Sebastián (Amigo de Ana, Rey del Parche) en La Troja (hace 30 min -> Capa 1 Feed de Ana)
    ('cccccccc-0000-0000-0000-000000000003', '10101010-1010-1010-1010-101010101010', 'bbbbbbbb-0000-0000-0000-000000000001',
     'https://images.unsplash.com/photo-1514525253161-7a46d19cd819?w=900&auto=format&fit=crop&q=80', 'checkins/10101010-1010-1010-1010-101010101010/troja_sebas.jpg',
     NOW() + INTERVAL '23 hours', NOW() - INTERVAL '30 minutes', false,
     ST_SetSRID(ST_MakePoint(-74.8075, 10.9982), 4326)::geography, true, true, true,
     25, true, NOW() - INTERVAL '30 minutes'),

    -- 4. Carlos (Amigo de Ana) en La Troja (hace 1 hora -> Capa 1 Feed de Ana)
    ('cccccccc-0000-0000-0000-000000000004', '22222222-2222-2222-2222-222222222222', 'bbbbbbbb-0000-0000-0000-000000000001',
     'https://images.unsplash.com/photo-1514933651103-005eec06c04b?w=900&auto=format&fit=crop&q=80', 'checkins/22222222-2222-2222-2222-222222222222/troja_carlos.jpg',
     NOW() + INTERVAL '23 hours', NOW() - INTERVAL '1 hour', false,
     ST_SetSRID(ST_MakePoint(-74.8075, 10.9982), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '1 hour'),

    -- 5. Andrés (No amigo de Ana, pero Ana SIGUE Caimán del Río -> Capa 2 Feed de Ana)
    ('cccccccc-0000-0000-0000-000000000005', '30303030-3030-3030-3030-303030303030', 'bbbbbbbb-0000-0000-0000-000000000007',
     'https://images.unsplash.com/photo-1414235077428-338989a2e8c0?w=900&auto=format&fit=crop&q=80', 'checkins/30303030-3030-3030-3030-303030303030/caiman_andres.jpg',
     NOW() + INTERVAL '22 hours', NOW() - INTERVAL '1 hour 15 minutes', false,
     ST_SetSRID(ST_MakePoint(-74.7890, 11.0125), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '1 hour 15 minutes'),

    -- 6. Mariana (No amiga de Ana, pero Ana SIGUE La Troja -> Capa 2 Feed de Ana)
    ('cccccccc-0000-0000-0000-000000000006', '40404040-4040-4040-4040-404040404040', 'bbbbbbbb-0000-0000-0000-000000000001',
     'https://images.unsplash.com/photo-1492684223066-81342ee5ff30?w=900&auto=format&fit=crop&q=80', 'checkins/40404040-4040-4040-4040-404040404040/troja_mariana.jpg',
     NOW() + INTERVAL '22 hours', NOW() - INTERVAL '2 hours', false,
     ST_SetSRID(ST_MakePoint(-74.8075, 10.9982), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '2 hours'),

    -- 7. Laura (Pública, no amiga de Ana, lugar no seguido por Ana -> Capa 3 Feed de Ana)
    ('cccccccc-0000-0000-0000-000000000007', '33333333-3333-3333-3333-333333333333', 'bbbbbbbb-0000-0000-0000-000000000008',
     'https://images.unsplash.com/photo-1504674900247-0877df9cc836?w=900&auto=format&fit=crop&q=80', 'checkins/33333333-3333-3333-3333-333333333333/narcobollo_laura.jpg',
     NOW() + INTERVAL '22 hours', NOW() - INTERVAL '1 hour 30 minutes', false,
     ST_SetSRID(ST_MakePoint(-74.8210, 11.0110), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '1 hour 30 minutes'),

    -- 8. Diego (Público, no amigo de Ana, lugar no seguido por Ana -> Capa 3 Feed de Ana)
    ('cccccccc-0000-0000-0000-000000000008', '50505050-5050-5050-5050-505050505050', 'bbbbbbbb-0000-0000-0000-000000000002',
     'https://images.unsplash.com/photo-1572116469696-31de0f17cc34?w=900&auto=format&fit=crop&q=80', 'checkins/50505050-5050-5050-5050-505050505050/bourbon_diego.jpg',
     NOW() + INTERVAL '21 hours', NOW() - INTERVAL '2 hours 45 minutes', false,
     ST_SetSRID(ST_MakePoint(-74.8115, 11.0065), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '2 hours 45 minutes'),

    -- 9. Valentina en Díscolo (Discoteca encendida hace 3 horas)
    ('cccccccc-0000-0000-0000-000000000009', '20202020-2020-2020-2020-202020202020', 'bbbbbbbb-0000-0000-0000-000000000004',
     'https://images.unsplash.com/photo-1545128485-c400e7702796?w=900&auto=format&fit=crop&q=80', 'checkins/20202020-2020-2020-2020-202020202020/discolo_valen.jpg',
     NOW() + INTERVAL '21 hours', NOW() - INTERVAL '3 hours', false,
     ST_SetSRID(ST_MakePoint(-74.8235, 11.0118), 4326)::geography, true, true, true,
     25, false, NOW() - INTERVAL '3 hours'),

    -- 10. Sofía en Bourbon St. Bar (Amiga de Ana, perfil privado)
    ('cccccccc-0000-0000-0000-000000000010', '55555555-5555-5555-5555-555555555555', 'bbbbbbbb-0000-0000-0000-000000000002',
     'https://images.unsplash.com/photo-1510812431401-41d2bd2722f3?w=900&auto=format&fit=crop&q=80', 'checkins/55555555-5555-5555-5555-555555555555/bourbon_sofia.jpg',
     NOW() + INTERVAL '21 hours', NOW() - INTERVAL '2 hours 15 minutes', false,
     ST_SetSRID(ST_MakePoint(-74.8115, 11.0065), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '2 hours 15 minutes'),

    -- 11. Ana en Gran Malecón (HACE 1 DÍA, FOTO EXPIRADA -> prueba placeholder visual "Foto expirada")
    ('cccccccc-0000-0000-0000-000000000011', '11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000014',
     'https://images.unsplash.com/photo-1507525428034-b723cf961d3e?w=900&auto=format&fit=crop&q=80', 'checkins/11111111-1111-1111-1111-111111111111/malecon_expired.jpg',
     NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 day', false,
     ST_SetSRID(ST_MakePoint(-74.7850, 11.0080), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '1 day'),

    -- 12. Ana en Caimán del Río (HACE 2 DÍAS, FOTO PERMANENTE -> historial de checkins)
    ('cccccccc-0000-0000-0000-000000000012', '11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000007',
     'https://images.unsplash.com/photo-1555396273-367ea4eb4db5?w=900&auto=format&fit=crop&q=80', 'checkins/11111111-1111-1111-1111-111111111111/caiman_perm.jpg',
     NULL, NOW() - INTERVAL '2 days', true,
     ST_SetSRID(ST_MakePoint(-74.7890, 11.0125), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '2 days'),

    -- 13. Carlos en Cucayo (HACE 3 DÍAS -> historial de checkins)
    ('cccccccc-0000-0000-0000-000000000013', '22222222-2222-2222-2222-222222222222', 'bbbbbbbb-0000-0000-0000-000000000009',
     'https://images.unsplash.com/photo-1504674900247-0877df9cc836?w=900&auto=format&fit=crop&q=80', 'checkins/22222222-2222-2222-2222-222222222222/cucayo.jpg',
     NOW() - INTERVAL '2 days', NOW() - INTERVAL '3 days', false,
     ST_SetSRID(ST_MakePoint(-74.8100, 11.0040), 4326)::geography, true, true, true,
     20, true, NOW() - INTERVAL '3 days'),

    -- 14. Sebastián en Díscolo (HACE 4 DÍAS, FOTO PERMANENTE -> historial)
    ('cccccccc-0000-0000-0000-000000000014', '10101010-1010-1010-1010-101010101010', 'bbbbbbbb-0000-0000-0000-000000000004',
     'https://images.unsplash.com/photo-1516450360452-9312f5e86fc7?w=900&auto=format&fit=crop&q=80', 'checkins/10101010-1010-1010-1010-101010101010/discolo_perm.jpg',
     NULL, NOW() - INTERVAL '4 days', true,
     ST_SetSRID(ST_MakePoint(-74.8235, 11.0118), 4326)::geography, true, true, true,
     30, true, NOW() - INTERVAL '4 days');

-- ── 5. Transacciones de Puntos (Auditoría Gamificada) ──────────────────────
-- Razones: CHECKIN, PLAN, REFERRAL
INSERT INTO point_transactions (tx_id, user_id, points, reason, reference_id, created_at) VALUES
    ('ffffffff-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 20, 'CHECKIN',  'cccccccc-0000-0000-0000-000000000001', NOW() - INTERVAL '35 minutes'),
    ('ffffffff-0000-0000-0000-000000000002', '20202020-2020-2020-2020-202020202020', 20, 'CHECKIN',  'cccccccc-0000-0000-0000-000000000002', NOW() - INTERVAL '20 minutes'),
    ('ffffffff-0000-0000-0000-000000000003', '10101010-1010-1010-1010-101010101010', 25, 'CHECKIN',  'cccccccc-0000-0000-0000-000000000003', NOW() - INTERVAL '30 minutes'),
    ('ffffffff-0000-0000-0000-000000000004', '22222222-2222-2222-2222-222222222222', 20, 'CHECKIN',  'cccccccc-0000-0000-0000-000000000004', NOW() - INTERVAL '1 hour'),
    ('ffffffff-0000-0000-0000-000000000005', '30303030-3030-3030-3030-303030303030', 20, 'CHECKIN',  'cccccccc-0000-0000-0000-000000000005', NOW() - INTERVAL '1 hour 15 minutes'),
    ('ffffffff-0000-0000-0000-000000000006', '11111111-1111-1111-1111-111111111111', 50, 'PLAN',     'dddddddd-0000-0000-0000-000000000004', NOW() - INTERVAL '2 days'),
    ('ffffffff-0000-0000-0000-000000000007', '55555555-5555-5555-5555-555555555555', 50, 'PLAN',     'dddddddd-0000-0000-0000-000000000004', NOW() - INTERVAL '2 days'),
    ('ffffffff-0000-0000-0000-000000000008', '11111111-1111-1111-1111-111111111111', 100, 'REFERRAL', '22222222-2222-2222-2222-222222222222', NOW() - INTERVAL '20 days');

-- ── 6. Seguir Lugares (Follows) ───────────────────────────────────────────
-- Determina qué check-ins aparecen en la Capa 2 del feed ("Lugares que sigues")
INSERT INTO follows (follower_id, place_id) VALUES
    -- Ana sigue La Troja, Caimán del Río, Café San Alberto y Puerta de Oro
    ('11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000001'),
    ('11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000007'),
    ('11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000011'),
    ('11111111-1111-1111-1111-111111111111', 'bbbbbbbb-0000-0000-0000-000000000021'),

    -- Carlos sigue La Troja, Bourbon St. Bar y Frogg Club
    ('22222222-2222-2222-2222-222222222222', 'bbbbbbbb-0000-0000-0000-000000000001'),
    ('22222222-2222-2222-2222-222222222222', 'bbbbbbbb-0000-0000-0000-000000000002'),
    ('22222222-2222-2222-2222-222222222222', 'bbbbbbbb-0000-0000-0000-000000000005'),

    -- Valentina sigue Caimán del Río y Díscolo
    ('20202020-2020-2020-2020-202020202020', 'bbbbbbbb-0000-0000-0000-000000000007'),
    ('20202020-2020-2020-2020-202020202020', 'bbbbbbbb-0000-0000-0000-000000000004'),

    -- Sebastián sigue La Troja, Gran Malecón y Díscolo
    ('10101010-1010-1010-1010-101010101010', 'bbbbbbbb-0000-0000-0000-000000000001'),
    ('10101010-1010-1010-1010-101010101010', 'bbbbbbbb-0000-0000-0000-000000000014'),
    ('10101010-1010-1010-1010-101010101010', 'bbbbbbbb-0000-0000-0000-000000000004'),

    -- Laura sigue Gran Malecón y Plaza de la Paz
    ('33333333-3333-3333-3333-333333333333', 'bbbbbbbb-0000-0000-0000-000000000014'),
    ('33333333-3333-3333-3333-333333333333', 'bbbbbbbb-0000-0000-0000-000000000015'),

    -- Sofía sigue Café San Alberto y Miura
    ('55555555-5555-5555-5555-555555555555', 'bbbbbbbb-0000-0000-0000-000000000011'),
    ('55555555-5555-5555-5555-555555555555', 'bbbbbbbb-0000-0000-0000-000000000010');

-- ── 7. Amistades y Solicitudes (Grafo Social) ─────────────────────────────
-- Regla estricta de la base de datos: user_id_a < user_id_b (orden lexicográfico)
-- status: 'accepted', 'pending', 'blocked'
INSERT INTO friendships (user_id_a, user_id_b, requester_id, status, created_at) VALUES
    -- 1. Sebastián (1010) y Ana (1111) -> Amigos confirmados
    ('10101010-1010-1010-1010-101010101010', '11111111-1111-1111-1111-111111111111',
     '10101010-1010-1010-1010-101010101010', 'accepted', NOW() - INTERVAL '60 days'),

    -- 2. Ana (1111) y Valentina (2020) -> Amigas confirmadas
    ('11111111-1111-1111-1111-111111111111', '20202020-2020-2020-2020-202020202020',
     '11111111-1111-1111-1111-111111111111', 'accepted', NOW() - INTERVAL '45 days'),

    -- 3. Ana (1111) y Carlos (2222) -> Amigos confirmados
    ('11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222',
     '11111111-1111-1111-1111-111111111111', 'accepted', NOW() - INTERVAL '30 days'),

    -- 4. Ana (1111) y Sofía (5555) -> Amigas confirmadas
    ('11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555555',
     '55555555-5555-5555-5555-555555555555', 'accepted', NOW() - INTERVAL '20 days'),

    -- 5. Camilo (6060) solicita amistad a Ana (1111) -> SOLICITUD ENTRANTE (Receptor: Ana)
    -- Aparece en /api/friend-requests/incoming de Ana. Permite probar Aceptar y Rechazar.
    ('11111111-1111-1111-1111-111111111111', '60606060-6060-6060-6060-606060606060',
     '60606060-6060-6060-6060-606060606060', 'pending',  NOW() - INTERVAL '3 hours'),

    -- 6. Ana (1111) solicita amistad a Diego (5050) -> SOLICITUD SALIENTE (Emisor: Ana)
    -- Aparece en /api/friend-requests/outgoing de Ana. Permite probar Cancelar solicitud.
    ('11111111-1111-1111-1111-111111111111', '50505050-5050-5050-5050-505050505050',
     '11111111-1111-1111-1111-111111111111', 'pending',  NOW() - INTERVAL '5 hours'),

    -- 7. Carlos (2222) y Laura (3333) -> Amigos confirmados
    ('22222222-2222-2222-2222-222222222222', '33333333-3333-3333-3333-333333333333',
     '22222222-2222-2222-2222-222222222222', 'accepted', NOW() - INTERVAL '15 days'),

    -- 8. Sebastián (1010) y Valentina (2020) -> Amigos confirmados
    ('10101010-1010-1010-1010-101010101010', '20202020-2020-2020-2020-202020202020',
     '10101010-1010-1010-1010-101010101010', 'accepted', NOW() - INTERVAL '40 days'),

    -- 9. Andrés (3030) y Mariana (4040) -> Amigos confirmados
    ('30303030-3030-3030-3030-303030303030', '40404040-4040-4040-4040-404040404040',
     '30303030-3030-3030-3030-303030303030', 'accepted', NOW() - INTERVAL '25 days'),

    -- 10. Carlos (2222) bloqueó a Pedro (6666) -> Usuario BLOQUEADO
    ('22222222-2222-2222-2222-222222222222', '66666666-6666-6666-6666-666666666666',
     '22222222-2222-2222-2222-222222222222', 'blocked',  NOW() - INTERVAL '10 days');

-- ── 8. Planes Sociales ("Parches") ─────────────────────────────────────────
-- Modalidades:
--   1 a 1 (INVITE): Con target_user_id (bandejas incoming/outgoing)
--   Abiertos (PUBLIC o FRIENDS): target_user_id = NULL (pines en el mapa /api/plans/discover)
-- Estados: PENDING, ACCEPTED, COMPLETED, FAILED, DECLINED
-- Plazos: 24, 48 o 72 horas
INSERT INTO plans (
    plan_id, creator_user_id, target_user_id, place_id,
    status, visibility, message,
    deadline_hours, deadline_at, completed_at, created_at
) VALUES
    -- Plan 1: Directo PENDIENTE de Carlos hacia Ana (Aparece en /api/plans/incoming de Ana)
    ('dddddddd-0000-0000-0000-000000000001', '22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111',
     'bbbbbbbb-0000-0000-0000-000000000001',
     'PENDING', 'INVITE', '¿Unas polas bien frías y salsa en La Troja este viernes?',
     24, NOW() + INTERVAL '20 hours', NULL, NOW() - INTERVAL '2 hours'),

    -- Plan 2: Directo PENDIENTE de Ana hacia Valentina (Aparece en /api/plans/outgoing de Ana)
    ('dddddddd-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111', '20202020-2020-2020-2020-202020202020',
     'bbbbbbbb-0000-0000-0000-000000000011',
     'PENDING', 'INVITE', '¿Nos vemos por la tarde en Café San Alberto a probar el nuevo Geisha?',
     48, NOW() + INTERVAL '44 hours', NULL, NOW() - INTERVAL '3 hours'),

    -- Plan 3: Directo ACEPTADO entre Sebastián y Ana (En curso con cuenta regresiva)
    ('dddddddd-0000-0000-0000-000000000003', '10101010-1010-1010-1010-101010101010', '11111111-1111-1111-1111-111111111111',
     'bbbbbbbb-0000-0000-0000-000000000007',
     'ACCEPTED', 'INVITE', 'Tarde de café y charla al atardecer frente al río en Caimán',
     24, NOW() + INTERVAL '18 hours', NULL, NOW() - INTERVAL '5 hours'),

    -- Plan 4: Directo CONCRETADO entre Sofía y Ana (Historial de planes completados)
    ('dddddddd-0000-0000-0000-000000000004', '55555555-5555-5555-5555-555555555555', '11111111-1111-1111-1111-111111111111',
     'bbbbbbbb-0000-0000-0000-000000000016',
     'COMPLETED', 'FRIENDS', 'Paseo cultural por el Parque del Caribe',
     48, NOW() - INTERVAL '1 day', NOW() - INTERVAL '2 days', NOW() - INTERVAL '5 days'),

    -- Plan 5: Directo EXPIRADO / NO CONCRETADO entre Carlos y Laura (Historial no concretados)
    ('dddddddd-0000-0000-0000-000000000005', '22222222-2222-2222-2222-222222222222', '33333333-3333-3333-3333-333333333333',
     'bbbbbbbb-0000-0000-0000-000000000002',
     'FAILED', 'INVITE', '¿Noche de jazz en Bourbon St.?',
     24, NOW() - INTERVAL '1 day', NULL, NOW() - INTERVAL '3 days'),

    -- Plan 6: Plan ABIERTO PÚBLICO ("Mesa Nómade" en Caimán del Río)
    -- Aparece como pin en el mapa (/api/plans/discover). Ana puede pulsar "Unirme al plan".
    ('dddddddd-0000-0000-0000-000000000006', '10101010-1010-1010-1010-101010101010', NULL,
     'bbbbbbbb-0000-0000-0000-000000000007',
     'PENDING', 'PUBLIC', 'Mesa Nómade: Coworking remoto, networking tech y café de media tarde. ¡Caigan!',
     48, NOW() + INTERVAL '40 hours', NULL, NOW() - INTERVAL '2 hours'),

    -- Plan 7: Plan ABIERTO DE AMIGOS en La Troja (Creado por Carlos, visible para Ana)
    ('dddddddd-0000-0000-0000-000000000007', '22222222-2222-2222-2222-222222222222', NULL,
     'bbbbbbbb-0000-0000-0000-000000000001',
     'PENDING', 'FRIENDS', 'Viernes salsero con los panas. Caigan después de las 7:00 pm.',
     48, NOW() + INTERVAL '42 hours', NULL, NOW() - INTERVAL '1 hour'),

    -- Plan 8: Plan ABIERTO PÚBLICO en Plaza de la Paz (Creado por Laura)
    ('dddddddd-0000-0000-0000-000000000008', '33333333-3333-3333-3333-333333333333', NULL,
     'bbbbbbbb-0000-0000-0000-000000000015',
     'PENDING', 'PUBLIC', 'Parchada en patines y helado al atardecer en la plaza.',
     24, NOW() + INTERVAL '22 hours', NULL, NOW() - INTERVAL '30 minutes');

-- ── 9. Participantes de Planes ────────────────────────────────────────────
-- roles: CREATOR, GUEST
-- status: INVITED, JOINED, COMPLETED, DECLINED
INSERT INTO plan_participants (plan_id, user_id, role, status, joined_at, completed_at) VALUES
    -- Plan 1 (Carlos invita a Ana)
    ('dddddddd-0000-0000-0000-000000000001', '22222222-2222-2222-2222-222222222222', 'CREATOR', 'JOINED',  NOW() - INTERVAL '2 hours', NULL),
    ('dddddddd-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', 'GUEST',   'INVITED', NOW() - INTERVAL '2 hours', NULL),

    -- Plan 2 (Ana invita a Valentina)
    ('dddddddd-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111', 'CREATOR', 'JOINED',  NOW() - INTERVAL '3 hours', NULL),
    ('dddddddd-0000-0000-0000-000000000002', '20202020-2020-2020-2020-202020202020', 'GUEST',   'INVITED', NOW() - INTERVAL '3 hours', NULL),

    -- Plan 3 (Sebastián y Ana - Aceptado en curso)
    ('dddddddd-0000-0000-0000-000000000003', '10101010-1010-1010-1010-101010101010', 'CREATOR', 'JOINED',  NOW() - INTERVAL '5 hours', NULL),
    ('dddddddd-0000-0000-0000-000000000003', '11111111-1111-1111-1111-111111111111', 'GUEST',   'JOINED',  NOW() - INTERVAL '4 hours', NULL),

    -- Plan 4 (Sofía y Ana - Concretado en el pasado)
    ('dddddddd-0000-0000-0000-000000000004', '55555555-5555-5555-5555-555555555555', 'CREATOR', 'COMPLETED', NOW() - INTERVAL '5 days', NOW() - INTERVAL '2 days'),
    ('dddddddd-0000-0000-0000-000000000004', '11111111-1111-1111-1111-111111111111', 'GUEST',   'COMPLETED', NOW() - INTERVAL '4 days', NOW() - INTERVAL '2 days'),

    -- Plan 5 (Carlos y Laura - Fallido)
    ('dddddddd-0000-0000-0000-000000000005', '22222222-2222-2222-2222-222222222222', 'CREATOR', 'JOINED',  NOW() - INTERVAL '3 days', NULL),
    ('dddddddd-0000-0000-0000-000000000005', '33333333-3333-3333-3333-333333333333', 'GUEST',   'INVITED', NOW() - INTERVAL '3 days', NULL),

    -- Plan 6 (Mesa abierta de Sebastián en Caimán del Río - Con participantes ya unidos)
    ('dddddddd-0000-0000-0000-000000000006', '10101010-1010-1010-1010-101010101010', 'CREATOR', 'JOINED', NOW() - INTERVAL '2 hours', NULL),
    ('dddddddd-0000-0000-0000-000000000006', '20202020-2020-2020-2020-202020202020', 'GUEST',   'JOINED', NOW() - INTERVAL '1 hour 45 minutes', NULL),
    ('dddddddd-0000-0000-0000-000000000006', '30303030-3030-3030-3030-303030303030', 'GUEST',   'JOINED', NOW() - INTERVAL '1 hour', NULL),

    -- Plan 7 (Mesa amigos de Carlos en La Troja)
    ('dddddddd-0000-0000-0000-000000000007', '22222222-2222-2222-2222-222222222222', 'CREATOR', 'JOINED', NOW() - INTERVAL '1 hour', NULL),

    -- Plan 8 (Mesa abierta de Laura en Plaza de la Paz)
    ('dddddddd-0000-0000-0000-000000000008', '33333333-3333-3333-3333-333333333333', 'CREATOR', 'JOINED', NOW() - INTERVAL '30 minutes', NULL);

-- ── 10. Moderación y Reportes de Prueba ────────────────────────────────────
-- target_type: 'PHOTO', 'USER', 'PLACE'
-- reason: 'INAPPROPRIATE_CONTENT', 'HARASSMENT', 'SPAM', 'FAKE_CONTENT', 'OTHER'
-- status: 'PENDING', 'REVIEWED', 'DISMISSED'
INSERT INTO reports (
    report_id, reporter_id, target_type, target_id,
    reason, comment, status, reviewed_at, reviewed_by
) VALUES
    -- Denuncia de usuario pendiente
    ('eeeeeeee-0000-0000-0000-000000000001', '33333333-3333-3333-3333-333333333333',
     'USER', '66666666-6666-6666-6666-666666666666',
     'HARASSMENT', 'Comportamiento insistente tras rechazar invitación.',
     'PENDING', NULL, NULL),

    -- Denuncia de foto revisada y desestimada
    ('eeeeeeee-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111',
     'PHOTO', 'cccccccc-0000-0000-0000-000000000009',
     'INAPPROPRIATE_CONTENT', 'Foto tomada en exteriores del local, no adentro.',
     'DISMISSED', NOW() - INTERVAL '1 day', '99999999-9999-9999-9999-999999999999'),

    -- Denuncia de lugar revisada
    ('eeeeeeee-0000-0000-0000-000000000003', '22222222-2222-2222-2222-222222222222',
     'PLACE', 'bbbbbbbb-0000-0000-0000-000000000006',
     'SPAM', 'Ubicación desfasada por obras en la vía.',
     'REVIEWED', NOW() - INTERVAL '12 hours', '99999999-9999-9999-9999-999999999999');

-- ── 11. Refresh Tokens (Sesiones activas de desarrollo) ─────────────────────
-- Permite que clientes de prueba refresquen token sin volver a ingresar credenciales
INSERT INTO refresh_tokens (token_id, user_id, token_hash, expires_at, revoked_at) VALUES
    -- Sesión activa de Ana
    ('77777777-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111',
     decode('e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855', 'hex'),
     NOW() + INTERVAL '30 days', NULL),
    -- Sesión activa de Sebastián
    ('77777777-0000-0000-0000-000000000002', '10101010-1010-1010-1010-101010101010',
     decode('2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae', 'hex'),
     NOW() + INTERVAL '30 days', NULL);

COMMIT;
