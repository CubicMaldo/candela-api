-- Seed de prueba para Barranquilla, Colombia
-- Ejecutar con: make shell-db y pegar este contenido

-- 1. Crear un negocio de prueba
INSERT INTO businesses (legal_name, contact_email, plan, is_verified)
VALUES ('Grupo Gastronómico BAQ', 'hola@baqfood.com', 'PREMIUM', true)
ON CONFLICT DO NOTHING;

-- 2. Insertar lugares icónicos de Barranquilla con coordenadas reales
-- Nota: ST_SetSRID(ST_MakePoint(longitud, latitud), 4326)::geography

DO $$
DECLARE
    bus_id UUID;
BEGIN
    SELECT business_id INTO bus_id FROM businesses LIMIT 1;

    -- Ventana al Mundo
    INSERT INTO places (name, description, category, location, geofence_meters, business_id, is_verified)
    VALUES (
        'Ventana al Mundo', 
        'Monumento icónico cerca de la zona industrial.', 
        'OTHER', 
        ST_SetSRID(ST_MakePoint(-74.8018, 11.0331), 4326)::geography,
        150, bus_id, true
    );

    -- Centro Comercial Viva
    INSERT INTO places (name, description, category, location, geofence_meters, business_id, is_verified)
    VALUES (
        'C.C. Viva Barranquilla', 
        'Centro comercial principal con gran zona de comidas.', 
        'STORE', 
        ST_SetSRID(ST_MakePoint(-74.8143, 11.0004), 4326)::geography,
        200, bus_id, true
    );

    -- Gran Malecón del Río
    INSERT INTO places (name, description, category, location, geofence_meters, business_id, is_verified)
    VALUES (
        'Gran Malecón', 
        'Lugar favorito para caminar al lado del Magdalena.', 
        'PARK', 
        ST_SetSRID(ST_MakePoint(-74.7770, 11.0022), 4326)::geography,
        200, bus_id, true
    );

    -- Universidad del Norte
    INSERT INTO places (name, description, category, location, geofence_meters, business_id, is_verified)
    VALUES (
        'Universidad del Norte', 
        'Campus universitario principal.', 
        'UNIVERSITY', 
        ST_SetSRID(ST_MakePoint(-74.8504, 11.0189), 4326)::geography,
        200, bus_id, true
    );

    -- Un bar en la 84 (Solo adultos)
    INSERT INTO places (name, description, category, location, geofence_meters, business_id, is_verified, minor_suitability)
    VALUES (
        'La Troja 84', 
        'Salsa y cultura local.', 
        'BAR', 
        ST_SetSRID(ST_MakePoint(-74.8141, 10.9935), 4326)::geography,
        100, bus_id, true, 'NO'
    );

END $$;
