BEGIN;

-- Remove the platform account created by the previous development seed.
DELETE FROM platform_users
WHERE email = 'owner@platform.test';

DO $$
DECLARE
    v_platform_user_id BIGINT;
    v_platform_role_id BIGINT;
BEGIN
    INSERT INTO platform_users (name, email, password_hash, is_active)
    VALUES (
        'Parag',
        'parag@platform.test',
        '$argon2id$v=19$m=65536,t=3,p=2$ZxxYxatTUcTneFKZWinsMQ$IQ35j1tP6Rp+Nwu+4DO+CRLuJx7GRr5+rxH6wRaP+70',
        true
    )
    ON CONFLICT (email) DO UPDATE
        SET name = EXCLUDED.name,
            password_hash = EXCLUDED.password_hash,
            is_active = true
    RETURNING id INTO v_platform_user_id;

    SELECT id INTO v_platform_role_id
    FROM platform_roles
    WHERE slug = 'product_owner';

    IF v_platform_role_id IS NULL THEN
        RAISE EXCEPTION 'product_owner platform role is missing; run migrations first';
    END IF;

    INSERT INTO platform_user_role_assignments (platform_user_id, platform_role_id)
    VALUES (v_platform_user_id, v_platform_role_id)
    ON CONFLICT DO NOTHING;
END $$;

COMMIT;
