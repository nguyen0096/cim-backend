-- The restaurant-admin role and its 16 p rows are removed in #150. A holder would be
-- left with a role that has no policy rows at all, which locks the account out of
-- every endpoint, so this is a guard rather than a rewrite: expected to find nothing,
-- and to stop the deploy rather than silently break an account if it does.

DO $$
DECLARE holders INT;
BEGIN
    SELECT count(*) INTO holders
    FROM users
    WHERE role = 'restaurant-admin' AND deleted_at IS NULL;

    IF holders > 0 THEN
        RAISE EXCEPTION 'restaurant-admin is still held by % user(s); reassign them before removing the role', holders;
    END IF;
END $$;
